package helper

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/utils"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strings"
)

//go:generate mockgen -source=rabbit_helper.go -destination=mock/helper.go
type RabbitHelper interface {
	CreateVHost(ctx context.Context) error
	CreateVHostAndReturnStatus(ctx context.Context) (int, error)
	DeleteVHost(ctx context.Context) error
	FormatCnnUrl(vHost string) string
	IsInstanceAvailable() error

	CreateOrUpdateUser(ctx context.Context, user string, password string) error

	GetExchange(ctx context.Context, exchange interface{}) (interface{}, error)
	GetVhostExchanges(ctx context.Context) ([]model.Exchange, error)
	CreateExchange(ctx context.Context, exchange interface{}) (interface{}, string, error)
	DeleteExchange(ctx context.Context, exchange interface{}) (interface{}, error)

	GetQueue(ctx context.Context, queue interface{}) (interface{}, error)
	GetVhostQueues(ctx context.Context) ([]model.Queue, error)
	CreateQueue(ctx context.Context, queue interface{}) (interface{}, string, error)
	DeleteQueue(ctx context.Context, queue interface{}) (interface{}, error)

	GetBinding(ctx context.Context, binding interface{}) (interface{}, error)
	CreateBinding(ctx context.Context, binding interface{}) (interface{}, string, error)
	DeleteBinding(ctx context.Context, binding interface{}) (interface{}, error)
	CreateNormalOrLazyBinding(ctx context.Context, binding map[string]interface{}) (map[string]interface{}, error)

	GetExchangeBinding(ctx context.Context, binding interface{}) (interface{}, error)
	CreateExchangeBinding(ctx context.Context, binding interface{}) (interface{}, string, error)
	DeleteExchangeBinding(ctx context.Context, binding interface{}) (interface{}, error)

	GetAllEntities(ctx context.Context) (model.RabbitEntities, error)

	GetAllExchanges(ctx context.Context) ([]interface{}, error)
	GetExchangesStartsWithString(ctx context.Context, prefix string) ([]interface{}, error)
	GetExchangeSourceBindings(ctx context.Context, exchange interface{}) ([]interface{}, error)

	GetPolicy(ctx context.Context, policy interface{}) (interface{}, error)
	CreatePolicy(ctx context.Context, policy interface{}) (interface{}, string, error)
	DeletePolicy(ctx context.Context, policy interface{}) (interface{}, error)

	CreateShovelForExportedQueue(ctx context.Context, vhosts []model.VHostRegistration, queue model.Queue) error
	CreateQueuesAndShovelsForExportedExchange(ctx context.Context, vhostAndVersion []model.VhostAndVersion, exchange model.Exchange) error
	DeleteShovelsForExportedVhost(ctx context.Context) error
	GetVhostShovels(ctx context.Context) ([]model.Shovel, error)
}

const shovelQueue = "-sq"

type RabbitHelperImpl struct {
	instance   model.RabbitInstance
	vhost      model.VHostRegistration
	httpHelper HttpHelper
	httpClient *resty.Client
}

var log logging.Logger

func init() {
	log = logging.GetLogger("rabbit-helper")
}

func NewRabbitHelper(rabbitInstance model.RabbitInstance, vhost model.VHostRegistration) RabbitHelperImpl {
	vhost.Password = vhost.GetDecodedPassword()
	httpHelper := NewHttpHelper()
	return NewRabbitHelperWithHttpHelper(rabbitInstance, vhost, httpHelper)
}

func NewRabbitHelperWithHttpHelper(rabbitInstance model.RabbitInstance, vhost model.VHostRegistration, httpHelper HttpHelper) RabbitHelperImpl {
	return RabbitHelperImpl{rabbitInstance, vhost, httpHelper, utils.NewRestyClient()}
}

type RabbitHttpError struct {
	Code          int // constant code from the specification
	ExpectedCodes []int
	Message       string
	Response      interface{} // description of the error
}

func (e RabbitHttpError) Error() string {
	return fmt.Sprintf("Error during request to RabbitMQ instance: Response code: (%d), Expected codes: (%v) Message: (%s), Response body: (%s)", e.Code, e.ExpectedCodes, e.Message, e.Response)
}

type RabbitHelperError struct {
	Message string
	Err     error
}

func (e RabbitHelperError) Error() string {
	return fmt.Sprintf("Rabbit helper error: (%v), message: (%v)", e.Err, e.Message)
}

var ErrNoVhostInRabbitInstance = errors.New("Vhost can't be found in RabbitMQ, but exists in MaaS registry. Maybe, you deleted vhost in RabbitMQ, but it still exists in MaaS - check if vhost still exists in RabbitMQ. Manual operations like this are not allowed in RabbitMQ, if you use MaaS. Please, delete this vhost correctly via MaaS delete vhost api")
var ErrNotAuthorizedRabbit = errors.New("Rabbit user in request to RabbitMQ was not authorized. Maybe, you deleted user of this vhost in RabbitMQ, but vhost still exists in MaaS - check if user still exists in RabbitMQ. Manual operations like this are not allowed in RabbitMQ, if you use MaaS. Please, delete this vhost correctly via MaaS delete vhost api")
var ErrNotFound = errors.New("Entity was not found")

type EntityAction func(ctx context.Context, entity interface{}) (interface{}, error)
type CreateEntityAction func(ctx context.Context, entity interface{}) (interface{}, string, error)

func (h RabbitHelperImpl) CreateVHost(ctx context.Context) error {
	_, err := h.CreateVHostAndReturnStatus(ctx)
	return err
}

func (h RabbitHelperImpl) CreateVHostAndReturnStatus(ctx context.Context) (int, error) {
	vhostName := h.vhost.Vhost
	vhostUsername := h.vhost.User
	vhostPassword := h.vhost.Password

	var code int

	log.InfoC(ctx, "Creating vhost in Rabbit: %+v", h.vhost)
	if response, err := h.httpHelper.DoRequest(ctx, resty.MethodPut,
		h.instance.ApiUrl+"/vhosts/"+vhostName,
		h.instance.User, h.instance.Password,
		map[string]interface{}{"name": h.vhost.Vhost},
		[]int{http.StatusCreated, http.StatusNoContent},
		fmt.Sprintf("Error creating vhost: %v", vhostName)); err != nil {
		log.ErrorC(ctx, "Error in creating vhost during DoRequest: %v", err.Error())
		return 0, err
	} else {
		code = response.StatusCode()
	}

	log.InfoC(ctx, "Create user: %v", vhostUsername)
	err := h.CreateOrUpdateUser(ctx, vhostUsername, vhostPassword)
	if err != nil {
		return 0, utils.LogError(log, ctx, "Error in creating/updating user during DoRequest: %w", err)
	}

	if _, err := h.httpHelper.DoRequest(ctx, resty.MethodPut,
		h.instance.ApiUrl+"/users/"+vhostUsername,
		h.instance.User, h.instance.Password,
		map[string]interface{}{"username": vhostUsername, "password": vhostPassword, "tags": "management"},
		[]int{http.StatusCreated, http.StatusOK, http.StatusNoContent},
		fmt.Sprintf("Error creating user: %v", vhostUsername)); err != nil {
		log.ErrorC(ctx, "Error in creating user during DoRequest: %v", err.Error())
		return 0, err
	}

	log.InfoC(ctx, "Grant user '%v' to vhostName '%v'", vhostUsername, vhostName)
	if _, err := h.httpHelper.DoRequest(ctx, resty.MethodPut,
		h.instance.ApiUrl+"/permissions/"+vhostName+"/"+vhostUsername,
		h.instance.User, h.instance.Password,
		map[string]interface{}{"vhost": vhostName, "username": vhostUsername, "configure": ".*", "write": ".*", "read": ".*"},
		[]int{http.StatusNoContent, http.StatusCreated, http.StatusOK},
		fmt.Sprintf("Error granting permissions user '%v' to '%v'", vhostUsername, vhostName)); err != nil {
		log.ErrorC(ctx, "Error in granting user during DoRequest: %v", err.Error())
		return 0, err
	}

	return code, nil
}

func (h RabbitHelperImpl) DeleteVHost(ctx context.Context) error {
	vhostName := h.vhost.Vhost
	vhostUsername := h.vhost.User

	log.InfoC(ctx, "Delete user: %v", vhostUsername)
	if _, err := h.httpHelper.DoRequest(ctx, resty.MethodDelete,
		h.instance.ApiUrl+"/users/"+vhostUsername,
		h.instance.User, h.instance.Password,
		nil,
		[]int{http.StatusNoContent, http.StatusNotFound},
		fmt.Sprintf("Error delete user: %v", vhostUsername)); err != nil {
		log.ErrorC(ctx, err.Error())
		return err
	}

	log.InfoC(ctx, "Delete vhostName: %v", vhostName)
	if _, err := h.httpHelper.DoRequest(ctx, resty.MethodDelete,
		h.instance.ApiUrl+"/vhosts/"+vhostName,
		h.instance.User, h.instance.Password,
		nil,
		[]int{http.StatusNoContent, http.StatusNotFound},
		fmt.Sprintf("Error delete vhostName: %v", vhostName)); err != nil {
		log.ErrorC(ctx, err.Error())
		return err
	}
	return nil
}

func (h RabbitHelperImpl) CreateOrUpdateUser(ctx context.Context, user string, password string) error {
	log.InfoC(ctx, "Create/update user: %v", user)
	_, err := h.httpHelper.DoRequest(ctx, resty.MethodPut,
		h.instance.ApiUrl+"/users/"+user,
		h.instance.User, h.instance.Password,
		map[string]interface{}{"username": user, "password": password, "tags": "management"},
		[]int{http.StatusCreated, http.StatusOK, http.StatusNoContent},
		fmt.Sprintf("Error creating/updating user: %v", user))
	if err != nil {
		return fmt.Errorf("error update user `%v' password: %w", user, err)
	}

	h.vhost.Password = password
	return nil
}

func (h RabbitHelperImpl) GetEntity(ctx context.Context, url string, toType interface{}) (interface{}, error) {
	return h.getEntity(ctx, url, h.vhost.User, h.vhost.Password, toType)
}

func (h RabbitHelperImpl) GetAdminEntity(ctx context.Context, url string, toType interface{}) (interface{}, error) {
	return h.getEntity(ctx, url, h.instance.User, h.instance.Password, toType)
}

func (h RabbitHelperImpl) getEntity(ctx context.Context, url, user, password string, toType interface{}) (interface{}, error) {
	log.InfoC(ctx, "Get entity with url: %v", url)
	if response, err := h.httpHelper.DoRequest(ctx, resty.MethodGet,
		fmt.Sprintf("%s/%s", h.instance.ApiUrl, url),
		user, password,
		nil,
		[]int{http.StatusOK, http.StatusNotFound},
		fmt.Sprintf("Error getting entity with url '%v', user '%v'", url, user)); err != nil {
		log.ErrorC(ctx, fmt.Sprintf("Error in getEntity during DoRequest: %v", err.Error()))
		return nil, err
	} else {
		if response.RawResponse.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		respBytes, err := io.ReadAll(response.RawResponse.Body)
		if err != nil {
			respBytes = response.Body()
		}

		if len(respBytes) != 0 {
			converted, err := h.convertToType(respBytes, toType)
			if err != nil {
				return nil, nil
			}
			if len(respBytes) == 2 {
				return nil, nil
			}
			return converted, nil
		} else {
			return nil, errors.New(fmt.Sprintf("status code is not 404, but body is nill for entity with url: %v", url))
		}
	}
}

// returns response, reason for update, error
func (h RabbitHelperImpl) CreateEntity(ctx context.Context, entity interface{}, url string, method string,
	getFunc EntityAction, deleteFunc EntityAction) (interface{}, string, error) {
	return h.createEntity(ctx, entity, url, method, h.vhost.User, h.vhost.Password, getFunc, deleteFunc)
}

func (h RabbitHelperImpl) CreateAdminEntity(ctx context.Context, entity interface{}, url, method string,
	getFunc EntityAction, deleteFunc EntityAction) (interface{}, string, error) {
	return h.createEntity(ctx, entity, url, method, h.instance.User, h.instance.Password, getFunc, deleteFunc)
}

func (h RabbitHelperImpl) createEntity(ctx context.Context, entity interface{}, url, method, user, password string,
	getFunc EntityAction, deleteFunc EntityAction) (interface{}, string, error) {
	log.InfoC(ctx, "Creating entity: %v", entity)
	if initResponse, err := h.httpHelper.DoRequest(ctx, method,
		fmt.Sprintf("%s/%s", h.instance.ApiUrl, url),
		user, password,
		entity,
		[]int{http.StatusOK, http.StatusCreated, http.StatusNoContent},
		fmt.Sprintf("Error creating entity: %v", entity)); err == nil {
		//in binding case adding properties_key from response header
		if strings.Contains(initResponse.Request.URL, "/api/bindings/") {
			location := initResponse.Header().Get("Location")
			pattern := regexp.MustCompile("^.*\\/(.*)$")
			groups := pattern.FindStringSubmatch(location)
			mapEntity := entity.(map[string]interface{})
			mapEntity["properties_key"] = groups[1]
		}

		if getFunc == nil {
			return nil, "", nil
		}
		resp, err := getFunc(ctx, entity)
		if err != nil || resp == nil {
			log.ErrorC(ctx, "Error getting entity: %v", entity)
			return nil, "", err
		}
		return resp, "", nil
	} else if deleteFunc != nil {
		rabbitErr, ok := err.(*RabbitHttpError)
		if !ok {
			log.ErrorC(ctx, "Problem during casting error to RabbitHttpError")
			return nil, "", err
		}
		if rabbitErr.Code == 400 && strings.Contains(fmt.Sprintf("%s", rabbitErr.Response), "inequivalent arg") {
			log.WarnC(ctx, "Entity %+v has changed and will be deleted and created again.", entity)
			_, err := deleteFunc(ctx, entity)
			if err != nil {
				log.ErrorC(ctx, "Error during entity deletion")
				return nil, "", err
			}
			response, _, err := h.createEntity(ctx, entity, url, method, user, password, getFunc, deleteFunc)
			if err != nil {
				log.ErrorC(ctx, "Error during entity recreation")
				return nil, "", err
			}
			reason, err := h.convertToType(rabbitErr.Response, map[string]interface{}{})
			if err != nil {
				log.ErrorC(ctx, "Error during converting to type: %v", err)
				return nil, "", err
			}
			reasonMap, ok := reason.(*map[string]interface{})
			if !ok {
				err := fmt.Errorf("error during conversion to *map[string]interface{} for '%+v'", reason)
				log.ErrorC(ctx, err.Error())
				return nil, "", err
			}
			return response, (*reasonMap)["reason"].(string), nil
		}
		log.ErrorC(ctx, err.Error())
		return nil, "", err
	} else {
		log.ErrorC(ctx, err.Error())
		return nil, "", err
	}
}

func (h RabbitHelperImpl) DeleteEntity(ctx context.Context, entity interface{}, url string) (interface{}, error) {
	return h.deleteEntity(ctx, entity, url, h.vhost.User, h.vhost.Password)
}

func (h RabbitHelperImpl) DeleteAdminEntity(ctx context.Context, entity interface{}, url string) (interface{}, error) {
	return h.deleteEntity(ctx, entity, url, h.instance.User, h.instance.Password)
}

func (h RabbitHelperImpl) deleteEntity(ctx context.Context, entity interface{}, url, user, password string) (interface{}, error) {
	log.InfoC(ctx, "Deleting entity: %v", entity)
	if response, err := h.httpHelper.DoRequest(ctx, resty.MethodDelete,
		fmt.Sprintf("%s/%s", h.instance.ApiUrl, url),
		user, password,
		nil,
		[]int{http.StatusNotFound, http.StatusNoContent},
		fmt.Sprintf("Error deleting url '%v' for entity : %v", url, entity)); err != nil {
		return nil, err
	} else {
		if response.RawResponse.StatusCode == http.StatusNoContent {
			return entity, nil
		} else if response.RawResponse.StatusCode == http.StatusNotFound {
			return nil, nil
		} else {
			return nil, errors.Errorf("Unexpected error during entity deletion, return status code: %v", response.RawResponse.StatusCode)
		}
	}
}

func (h RabbitHelperImpl) GetExchange(ctx context.Context, exchange interface{}) (interface{}, error) {
	log.DebugC(ctx, "Getting exchange: %v", exchange)
	exchangeName, err := utils.ExtractName(exchange)
	if err != nil {
		log.ErrorC(ctx, "ExchangeType entity '%v' doesn't have name field", exchange)
		return nil, err
	}
	url := fmt.Sprintf("exchanges/%s/%s", h.vhost.Vhost, exchangeName)
	return h.GetEntity(ctx, url, map[string]interface{}{})
}

func (h RabbitHelperImpl) CreateExchange(ctx context.Context, exchange interface{}) (interface{}, string, error) {
	log.InfoC(ctx, "Creating exchange: %v", exchange)
	exchangeName, err := utils.ExtractName(exchange)
	if err != nil {
		log.ErrorC(ctx, "ExchangeType entity '%v' doesn't have name field", exchange)
		return nil, "", err
	}
	url := fmt.Sprintf("exchanges/%s/%s", h.vhost.Vhost, exchangeName)
	return h.CreateEntity(ctx, exchange, url, http.MethodPut, h.GetExchange, h.DeleteExchange)
}

func (h RabbitHelperImpl) DeleteExchange(ctx context.Context, exchange interface{}) (interface{}, error) {
	log.InfoC(ctx, "Deleting exchange: %v", exchange)
	exchangeName, err := utils.ExtractName(exchange)
	if err != nil {
		log.ErrorC(ctx, "ExchangeType entity '%v' doesn't have name field", exchange)
		return nil, err
	}
	url := fmt.Sprintf("exchanges/%s/%s", h.vhost.Vhost, exchangeName)
	return h.DeleteEntity(ctx, exchange, url)
}

func (h RabbitHelperImpl) GetQueue(ctx context.Context, queue interface{}) (interface{}, error) {
	log.InfoC(ctx, "Get queue: %v", queue)
	queueName, err := utils.ExtractName(queue)
	if err != nil {
		log.ErrorC(ctx, "queue entity '%v' doesn't have name field", queue)
		return nil, err
	}
	url := fmt.Sprintf("queues/%s/%s", h.vhost.Vhost, queueName)
	return h.GetEntity(ctx, url, map[string]interface{}{})
}

func (h RabbitHelperImpl) CreateQueue(ctx context.Context, queue interface{}) (interface{}, string, error) {
	log.InfoC(ctx, "Creating queue: %v", queue)
	queueName, err := utils.ExtractName(queue)
	if err != nil {
		log.ErrorC(ctx, "queue entity '%v' doesn't have name field", queue)
		return nil, "", err
	}
	url := fmt.Sprintf("queues/%s/%s", h.vhost.Vhost, queueName)
	return h.CreateEntity(ctx, queue, url, http.MethodPut, h.GetQueue, h.DeleteQueue)
}

func (h RabbitHelperImpl) DeleteQueue(ctx context.Context, queue interface{}) (interface{}, error) {
	log.InfoC(ctx, "Deleting queue: %v", queue)
	queueName, err := utils.ExtractName(queue)
	if err != nil {
		log.ErrorC(ctx, "queue entity '%v' doesn't have name field", queue)
		return nil, err
	}
	url := fmt.Sprintf("queues/%s/%s", h.vhost.Vhost, queueName)
	return h.DeleteEntity(ctx, queue, url)
}

func (h RabbitHelperImpl) GetBinding(ctx context.Context, binding interface{}) (interface{}, error) {
	log.InfoC(ctx, "Get binding: %v", binding)
	source, destination, err := utils.ExtractSourceAndDestination(binding)
	if err != nil {
		log.ErrorC(ctx, "binding entity %v doesn't have either source or destination field", binding)
		return nil, err
	}
	propKey, err := utils.ExtractPropKey(binding)
	if err != nil {
		log.ErrorC(ctx, "binding entity %v doesn't have properties_key field", binding)
		return nil, err
	}
	url := fmt.Sprintf("bindings/%s/e/%s/q/%s/%s", h.vhost.Vhost, source, destination, propKey)
	return h.GetEntity(ctx, url, map[string]interface{}{})
}

func (h RabbitHelperImpl) GetExchangeBinding(ctx context.Context, binding interface{}) (interface{}, error) {
	log.InfoC(ctx, "Get exchange binding: %v", binding)
	source, destination, err := utils.ExtractSourceAndDestination(binding)
	if err != nil {
		log.ErrorC(ctx, "binding entity '%v' doesn't have either source or destination field", binding)
		return nil, err
	}
	url := fmt.Sprintf("bindings/%s/e/%s/e/%s", h.vhost.Vhost, source, destination)
	return h.GetEntity(ctx, url, []map[string]interface{}{})
}

func (h RabbitHelperImpl) CreateBinding(ctx context.Context, binding interface{}) (interface{}, string, error) {
	log.InfoC(ctx, "Creating binding: %v", binding)
	source, destination, err := utils.ExtractSourceAndDestination(binding)
	if err != nil {
		log.ErrorC(ctx, "binding entity '%v' doesn't have either source or destination field", binding)
		return nil, "", err
	}
	url := fmt.Sprintf("bindings/%s/e/%s/q/%s", h.vhost.Vhost, source, destination)
	result, _, err := h.CreateEntity(ctx, binding, url, http.MethodPost, h.GetBinding, nil)
	if err != nil {
		rabbitErr, ok := err.(*RabbitHttpError)
		if ok {
			if rabbitErr.Code == 404 {
				err = RabbitHelperError{
					Message: fmt.Sprintf("Source or destination was not found for binding: %+v", binding),
					Err:     ErrNotFound,
				}
			}
		}
	}

	return result, "", err
}

func (h RabbitHelperImpl) CreateNormalOrLazyBinding(ctx context.Context, binding map[string]interface{}) (map[string]interface{}, error) {
	log.InfoC(ctx, "Creating normal or lazy binding: %v", binding)
	source, destination, err := utils.ExtractSourceAndDestination(binding)
	if err != nil {
		log.ErrorC(ctx, "binding entity '%v' doesn't have either source or destination field", binding)
		return nil, err
	}
	url := fmt.Sprintf("bindings/%s/e/%s/q/%s", h.vhost.Vhost, source, destination)

	// queue existence checked during verification. we allow exchange to not exist
	initResponse, err := h.httpHelper.DoRequest(ctx, http.MethodPost,
		fmt.Sprintf("%s/%s", h.instance.ApiUrl, url),
		h.vhost.User, h.vhost.Password,
		binding,
		[]int{http.StatusOK, http.StatusCreated, http.StatusNoContent, http.StatusNotFound},
		fmt.Sprintf("Error creating lazy or normal binding: %v", binding))

	if err != nil {
		log.ErrorC(ctx, "Error during DoRequest while in CreateNormalOrLazyBinding: %v", err)
		return nil, err
	}

	createdBinding := make(map[string]interface{})
	for k, v := range binding {
		createdBinding[k] = v
	}

	location := initResponse.Header().Get("Location")
	if location != "" {
		pattern := regexp.MustCompile("^.*\\/(.*)$")
		groups := pattern.FindStringSubmatch(location)
		createdBinding["properties_key"] = groups[1]
		return createdBinding, nil
	} else {
		return nil, nil
	}
}

func (h RabbitHelperImpl) CreateExchangeBinding(ctx context.Context, binding interface{}) (interface{}, string, error) {
	log.InfoC(ctx, "Creating exchange binding: %v", binding)
	source, destination, err := utils.ExtractSourceAndDestination(binding)
	if err != nil {
		log.ErrorC(ctx, "binding entity '%v' doesn't have either source or destination field", binding)
		return nil, "", err
	}
	url := fmt.Sprintf("bindings/%s/e/%s/e/%s", h.vhost.Vhost, source, destination)
	result, _, err := h.CreateEntity(ctx, binding, url, http.MethodPost, h.GetExchangeBinding, nil)
	return result, "", err
}

func (h RabbitHelperImpl) DeleteBinding(ctx context.Context, binding interface{}) (interface{}, error) {
	log.InfoC(ctx, "Deleting binding: %v", binding)
	source, destination, err := utils.ExtractSourceAndDestination(binding)
	if err != nil {
		log.ErrorC(ctx, "binding entity '%v' doesn't have source or destination field", binding)
		return nil, err
	}
	propKey, err := utils.ExtractPropKey(binding)
	if err != nil {
		log.ErrorC(ctx, "binding entity %v doesn't have properties_key field", binding)
		return nil, err
	}
	url := fmt.Sprintf("bindings/%s/e/%s/q/%s/%s", h.vhost.Vhost, source, destination, propKey)
	return h.DeleteEntity(ctx, binding, url)
}

func (h RabbitHelperImpl) DeleteExchangeBinding(ctx context.Context, binding interface{}) (interface{}, error) {
	log.InfoC(ctx, "Deleting exchange binding: %v", binding)
	source, destination, err := utils.ExtractSourceAndDestination(binding)
	if err != nil {
		log.ErrorC(ctx, "binding entity '%v' doesn't have source or destination field", binding)
		return nil, err
	}
	propKey, err := utils.ExtractPropKey(binding)
	if err != nil {
		log.ErrorC(ctx, "binding entity %v doesn't have properties_key field", binding)
		return nil, err
	}
	url := fmt.Sprintf("bindings/%s/e/%s/e/%s/%s", h.vhost.Vhost, source, destination, propKey)
	return h.DeleteEntity(ctx, binding, url)
}

func (h RabbitHelperImpl) GetAllEntities(ctx context.Context) (model.RabbitEntities, error) {
	var entities model.RabbitEntities
	log.InfoC(ctx, "Get exchanges")
	url := fmt.Sprintf("exchanges/%s", h.vhost.Vhost)
	result, err := h.GetEntity(ctx, url, []interface{}{})
	if err != nil {
		rabbitErr, ok := err.(*RabbitHttpError)
		if ok {
			if rabbitErr.Code == 401 {
				err := RabbitHelperError{
					Message: fmt.Sprintf("User was not authorized for vhost: %+v", h.vhost),
					Err:     ErrNotAuthorizedRabbit,
				}
				log.ErrorC(ctx, err.Error())
				return entities, err
			}
		}
		log.ErrorC(ctx, "error while requesting exchanges for vhost '%+v': %v", h.vhost, err)
		return entities, err
	}
	if result != nil {
		_, ok := result.(*[]interface{})
		if !ok {
			err := fmt.Errorf("error during conversion exchanges to *[]interface{} for '%+v'", result)
			log.ErrorC(ctx, err.Error())
			return entities, err
		}
		entities.Exchanges = *result.(*[]interface{})
	} else {
		err := RabbitHelperError{
			Message: fmt.Sprintf("No exchanges in RabbitMQ for vhost: %+v", h.vhost),
			Err:     err,
		}
		log.ErrorC(ctx, err.Error())
		return entities, err
	}

	log.InfoC(ctx, "Get queues")
	url = fmt.Sprintf("queues/%s", h.vhost.Vhost)
	result, err = h.GetEntity(ctx, url, []interface{}{})
	if err != nil {
		log.ErrorC(ctx, "error while requesting queues for vhost '%+v': %v", h.vhost, err)
		return entities, err
	}
	if result != nil {
		_, ok := result.(*[]interface{})
		if !ok {
			err := fmt.Errorf("error during conversion queues to *[]interface{} for '%+v'", result)
			log.ErrorC(ctx, err.Error())
			return entities, err
		}
		entities.Queues = *result.(*[]interface{})
	}
	log.InfoC(ctx, "Get bindings")
	url = fmt.Sprintf("bindings/%s", h.vhost.Vhost)
	result, err = h.GetEntity(ctx, url, []interface{}{})
	if err != nil {
		log.ErrorC(ctx, "error while requesting bindings for vhost '%+v': %v", h.vhost, err)
		return entities, err
	}
	if result != nil {
		_, ok := result.(*[]interface{})
		if !ok {
			err := fmt.Errorf("error during conversion bindings to *[]interface{} for '%+v'", result)
			log.ErrorC(ctx, err.Error())
			return entities, err
		}
		entities.Bindings = *result.(*[]interface{})
	}
	return entities, err
}

func (h RabbitHelperImpl) GetAllExchanges(ctx context.Context) ([]interface{}, error) {
	log.InfoC(ctx, "GetAllExchanges of vhost with name '%v'", h.vhost.Vhost)
	url := fmt.Sprintf("exchanges/%s", h.vhost.Vhost)
	result, err := h.GetEntity(ctx, url, []interface{}{})
	if err != nil {
		log.ErrorC(ctx, "error while requesting exchanges: %v", err)
		return nil, err
	}
	if result == nil {
		return nil, RabbitHelperError{
			Message: fmt.Sprintf("No exchanges in RabbitMQ for vhost: %+v", h.vhost.Vhost),
			Err:     errors.New("No exchanges in RabbitMQ for vhost"),
		}
	}
	_, ok := result.(*[]interface{})
	if !ok {
		err := fmt.Errorf("error during conversion all exchanges to *[]interface{} for '%+v'", result)
		log.ErrorC(ctx, err.Error())
		return nil, err
	}
	return *result.(*[]interface{}), nil
}

func (h RabbitHelperImpl) GetExchangesStartsWithString(ctx context.Context, prefix string) ([]interface{}, error) {
	log.InfoC(ctx, "Get exchanges")
	url := fmt.Sprintf("exchanges/%s", h.vhost.Vhost)
	result, err := h.GetEntity(ctx, url, []interface{}{})
	if err != nil {
		log.ErrorC(ctx, "error while requesting exchanges: %v", err)
		return nil, err
	}
	var exchanges []interface{}

	for _, exchange := range *result.(*[]interface{}) {
		exchangeName, err := utils.ExtractName(exchange)
		if err != nil {
			log.WarnC(ctx, "ExchangeType entity %v doesn't have name field", exchange)
			exchangeName = ""
		}
		if strings.HasPrefix(exchangeName, prefix) {
			exchanges = append(exchanges, exchange)
		}
	}
	return exchanges, nil
}

func (h RabbitHelperImpl) GetExchangeSourceBindings(ctx context.Context, exchange interface{}) ([]interface{}, error) {
	log.InfoC(ctx, "Get bindings of exchange: %v", exchange)
	exchangeName, err := utils.ExtractName(exchange)
	if err != nil {
		log.ErrorC(ctx, "ExchangeType entity %v doesn't have name field", exchange)
		return nil, err
	}
	url := fmt.Sprintf("exchanges/%s/%s/bindings/source", h.vhost.Vhost, exchangeName)
	//bindings, err := h.GetEntity(ctx, url, []map[string]interface{}{})
	bindings, err := h.GetEntity(ctx, url, []interface{}{})
	log.InfoC(ctx, fmt.Sprintf("bindings of exchange '%v' are: %v", exchangeName, bindings))
	if bindings != nil {
		_, ok := bindings.(*[]interface{})
		if !ok {
			err := fmt.Errorf("error during conversion exchange source bindings to *[]interface{} for '%+v'", bindings)
			log.ErrorC(ctx, err.Error())
			return nil, err
		}
		return *bindings.(*[]interface{}), err
	} else {
		return nil, nil
	}
}

func (h RabbitHelperImpl) GetPolicy(ctx context.Context, policy interface{}) (interface{}, error) {
	log.InfoC(ctx, "Get policy: %v", policy)
	policyName, err := utils.ExtractName(policy)
	if err != nil {
		log.ErrorC(ctx, "policy entity %v doesn't have name field", policy)
		return nil, err
	}
	url := fmt.Sprintf("policies/%s/%s", h.vhost.Vhost, policyName)
	return h.GetAdminEntity(ctx, url, map[string]interface{}{})
}

// TODO: validate allowed policies, restrict user management policies
func (h RabbitHelperImpl) CreatePolicy(ctx context.Context, policy interface{}) (interface{}, string, error) {
	log.InfoC(ctx, "Creating policy: %v", policy)
	policyName, err := utils.ExtractName(policy)
	if err != nil {
		log.ErrorC(ctx, "policy entity %v doesn't have name field", policy)
		return nil, "", err
	}
	url := fmt.Sprintf("policies/%s/%s", h.vhost.Vhost, policyName)
	result, _, err := h.CreateAdminEntity(ctx, policy, url, http.MethodPut, h.GetPolicy, nil)
	return result, "", err
}

func (h RabbitHelperImpl) DeletePolicy(ctx context.Context, policy interface{}) (interface{}, error) {
	log.InfoC(ctx, "Deleting policy: %v", policy)
	policyName, err := utils.ExtractName(policy)
	if err != nil {
		log.ErrorC(ctx, "policy entity %v doesn't have name field", policy)
		return nil, err
	}
	url := fmt.Sprintf("policies/%s/%s", h.vhost.Vhost, policyName)
	return h.DeleteAdminEntity(ctx, policy, url)
}

func (h RabbitHelperImpl) CreateShovelForExportedQueue(ctx context.Context, vhosts []model.VHostRegistration, queue model.Queue) error {
	queueName, err := utils.ExtractName(queue)
	if err != nil {
		return utils.LogError(log, ctx, "queue entity '%v' doesn't have name field", queue)
	}

	address := h.instance.AmqpUrl
	address = strings.TrimLeft(address, "amqp://")
	address = strings.TrimLeft(address, "amqps://")

	for _, vhost := range vhosts {
		log.InfoC(ctx, "Creating shovel for queue '%v' and classifier '%v'", queueName, vhost.Classifier)
		shovel := model.Shovel{
			Value: model.ShovelValue{
				SrcProtocol:  "amqp091",
				SrcUri:       fmt.Sprintf("amqp://%v:%v@%v/%v", vhost.User, vhost.GetDecodedPassword(), address, vhost.Vhost),
				SrcQueue:     queueName,
				DestProtocol: "amqp091",
				DestUri:      fmt.Sprintf("amqp://%v:%v@%v/%v", h.vhost.User, h.vhost.GetDecodedPassword(), address, h.vhost.Vhost),
				DestQueue:    queueName,
			},
		}

		url := fmt.Sprintf("parameters/shovel/%s/%s", h.vhost.Vhost, queueName+"-"+vhost.Namespace+"-exported")
		_, _, err = h.CreateAdminEntity(ctx, shovel, url, http.MethodPut, nil, nil)
		if err != nil {
			return utils.LogError(log, ctx, "error during CreateAdminEntity: %w", err)
		}
	}

	return nil
}

func (h RabbitHelperImpl) CreateQueuesAndShovelsForExportedExchange(ctx context.Context, vhostsAndVersion []model.VhostAndVersion, exchange model.Exchange) error {
	exchangeName, err := utils.ExtractName(exchange)
	if err != nil {
		return utils.LogError(log, ctx, "exchange entity '%v' doesn't have name field", exchange)
	}

	address := h.instance.AmqpUrl
	address = strings.TrimLeft(address, "amqp://")
	address = strings.TrimLeft(address, "amqps://")

	for _, vhostAndVersion := range vhostsAndVersion {

		queueName := exchangeName + "-" + vhostAndVersion.Vhost.Namespace + shovelQueue

		_, _, err = h.CreateQueue(ctx, model.Queue{"name": queueName, "durable": true})
		if err != nil {
			return utils.LogError(log, ctx, "error during CreateQueue while in CreateQueuesAndShovelsForExportedExchange, queue name: %v, err: %w", queueName, err)
		}

		//bind vr to new versioned exchange
		veBindingArguments := map[string]interface{}{
			"version": vhostAndVersion.Version,
		}
		veBinding := map[string]interface{}{
			"source":      exchangeName,
			"destination": queueName,
			"arguments":   veBindingArguments,
			"routing_key": "*",
		}
		_, _, err = h.CreateBinding(ctx, veBinding)
		if err != nil {
			return utils.LogError(log, ctx, "error during CreateBinding while in CreateQueuesAndShovelsForExportedExchange, queue name: %v, err: %w", queueName, err)
		}

		log.InfoC(ctx, "Creating shovel for exchange queue '%v' and classifier '%v'", queueName, vhostAndVersion.Vhost.Classifier)
		shovel := model.Shovel{
			Value: model.ShovelValue{
				SrcProtocol:  "amqp091",
				SrcUri:       fmt.Sprintf("amqp://%v:%v@%v/%v", h.vhost.User, h.vhost.GetDecodedPassword(), address, h.vhost.Vhost),
				SrcQueue:     queueName,
				DestProtocol: "amqp091",
				DestUri:      fmt.Sprintf("amqp://%v:%v@%v/%v", vhostAndVersion.Vhost.User, vhostAndVersion.Vhost.GetDecodedPassword(), address, vhostAndVersion.Vhost.Vhost),
				DestExchange: exchangeName,
			},
		}

		url := fmt.Sprintf("parameters/shovel/%s/%s", h.vhost.Vhost, queueName+"-"+vhostAndVersion.Vhost.Namespace+"-exported")
		_, _, err = h.CreateAdminEntity(ctx, shovel, url, http.MethodPut, nil, nil)
		if err != nil {
			return utils.LogError(log, ctx, "error during CreateAdminEntity while in CreateQueuesAndShovelsForExportedExchange: %w", err)
		}
	}

	return nil
}

func (h RabbitHelperImpl) DeleteShovelsForExportedVhost(ctx context.Context) error {
	url := fmt.Sprintf("parameters/shovel/%s", h.vhost.Vhost)
	shovels, err := h.GetAdminEntity(ctx, url, []model.Shovel{})
	if err != nil {
		return utils.LogError(log, ctx, "error during GetAdminEntity while in DeleteShovelsForExportedVhost: %w", err)
	}

	if shovels == nil {
		return nil
	}
	shovelsConverted := shovels.(*[]model.Shovel)

	for _, shovel := range *shovelsConverted {
		url = fmt.Sprintf("parameters/shovel/%s/%s", h.vhost.Vhost, shovel.Name)
		_, err = h.DeleteAdminEntity(ctx, nil, url)
		if err != nil {
			return utils.LogError(log, ctx, "error during DeleteAdminEntity while in DeleteShovelsForExportedVhost in vhost '%s' for shovel named '%s': %w", h.vhost.Vhost, shovel.Name, err)
		}
	}

	return nil
}

func (h RabbitHelperImpl) GetVhostShovels(ctx context.Context) ([]model.Shovel, error) {
	url := fmt.Sprintf("parameters/shovel/%s", h.vhost.Vhost)
	shovels, err := h.GetAdminEntity(ctx, url, []model.Shovel{})
	if err != nil {
		return nil, utils.LogError(log, ctx, "error during DeleteAdminEntity: %w", err)
	}

	if shovels == nil {
		return nil, nil
	}
	shovelsConverted := shovels.(*[]model.Shovel)

	return *shovelsConverted, nil
}

func (h RabbitHelperImpl) GetVhostQueues(ctx context.Context) ([]model.Queue, error) {
	url := fmt.Sprintf("queues/%s", h.vhost.Vhost)
	queues, err := h.GetAdminEntity(ctx, url, []model.Queue{})
	if err != nil {
		return nil, utils.LogError(log, ctx, "error during GetAdminEntity: %w", err)
	}

	if queues == nil {
		return nil, nil
	}
	queuesConverted := queues.(*[]model.Queue)
	return *queuesConverted, nil
}

func (h RabbitHelperImpl) GetVhostExchanges(ctx context.Context) ([]model.Exchange, error) {
	url := fmt.Sprintf("exchanges/%s", h.vhost.Vhost)
	exchanges, err := h.GetAdminEntity(ctx, url, []model.Exchange{})
	if err != nil {
		return nil, utils.LogError(log, ctx, "error during GetAdminEntity: %w", err)
	}

	if exchanges == nil {
		return nil, nil
	}
	exchangesConverted := exchanges.(*[]model.Exchange)
	return *exchangesConverted, nil
}

func (h RabbitHelperImpl) FormatCnnUrl(vHost string) string {
	return fmt.Sprintf("%s/%s", h.instance.AmqpUrl, vHost)
}

func (h RabbitHelperImpl) convertToType(obj interface{}, toType interface{}) (interface{}, error) {
	jsonResp := reflect.New(reflect.TypeOf(toType)).Interface()
	objByte, ok := obj.([]byte)
	if !ok {
		err2 := json.Unmarshal(obj.([]uint8), &jsonResp)
		return jsonResp, err2
	} else {
		err2 := json.Unmarshal(objByte, &jsonResp)
		return jsonResp, err2
	}
}

func (h RabbitHelperImpl) IsInstanceAvailable() error {
	res, err := h.httpClient.R().
		SetBasicAuth(h.instance.User, h.instance.Password).
		Get(h.instance.ApiUrl + "/nodes")

	if err != nil {
		return err
	}

	if res.StatusCode() != 200 {
		return fmt.Errorf("rabbitmq api not ready: %s", res.Status())
	}

	return nil
}

func IsInstanceAvailable(rabbitInstance *model.RabbitInstance) error {
	rabbitHelper := NewRabbitHelper(*rabbitInstance, model.VHostRegistration{})
	return rabbitHelper.IsInstanceAvailable()
}
