package utils

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/go-resty/resty/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/netcracker/qubership-core-lib-go/v3/context-propagation/baseproviders/xrequestid"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"maas/maas-service/msg"
	"math/rand"
	"os"
	"path"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"
)

var log = logging.GetLogger("utils")

const RequestIdPropertyName = "requestId"

type WaitGroupWithTimeout struct {
	sync.WaitGroup
}

func (wg *WaitGroupWithTimeout) Wait(timeout time.Duration) bool {
	done := make(chan struct{})

	go func() {
		defer close(done)
		wg.WaitGroup.Wait()
	}()

	select {
	case <-done:
		return true

	case <-time.After(timeout):
		return false
	}
}

func CreateContextFromString(rId string) context.Context {
	return context.WithValue(context.Background(), RequestIdPropertyName, rId)
}
func GetBasicAuth(fiberCtx *fiber.Ctx) (string, SecretString, error) {
	var basicAuthPrefix = []byte("Basic ")
	var username string
	var password SecretString
	auth := fiberCtx.Request().Header.Peek(fiber.HeaderAuthorization)

	if len(auth) == 0 {
		return "", "", fmt.Errorf("header `%v' is empty: %w", fiber.HeaderAuthorization, msg.AuthError)
	}

	if !bytes.HasPrefix(auth, basicAuthPrefix) {
		return "", "", fmt.Errorf("not a basic auth, should have prefix 'Basic': %w", msg.AuthError)
	}

	// Check credentials
	payload, err := base64.StdEncoding.DecodeString(string(auth[len(basicAuthPrefix):]))
	if err != nil {
		return "", "", fmt.Errorf("error during decoding auth string")
	}

	pair := bytes.SplitN(payload, []byte(":"), 2)
	if len(pair) != 2 {
		return "", "", fmt.Errorf("credentials are requiered for basic auth, check that creds are in format 'username:password', decoded username is '%v': %w", pair[0], msg.AuthError)
	}
	username = string(pair[0])
	password = SecretString(pair[1])
	if username == "" || password == "" {
		return "", "", fmt.Errorf("credentials are requiered for basic auth, username or password is empty, decoded username is '%s': %w", username, msg.AuthError)
	}

	return username, password, nil
}

func CompactUuid() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}

func RetryValue[T any](ctx context.Context,
	timeout time.Duration,
	delayBetweenAttempts time.Duration,
	operation func(ctx context.Context) (T, error)) (T, error) {

	var err error
	var result T
	// try first, may be there is no need for retry after timeout?
	result, err = operation(ctx)
	if err == nil {
		log.DebugC(ctx, "Operation completed successfully without retry")
		return result, nil
	} else {
		log.WarnC(ctx, "Operation failed with error: %v. Retry operation", err)
	}

	// start retry
	ctx, timoutCancel := context.WithTimeout(ctx, timeout)
	defer timoutCancel()
	ticker := time.NewTicker(delayBetweenAttempts)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if err != nil {
				return result, fmt.Errorf("retrying timeout reached: root cause: %w", err)
			} else {
				return result, ctx.Err()
			}
		case <-ticker.C:
			result, err = operation(ctx)
			if err == nil {
				log.DebugC(ctx, "Operation completed successfully")
				return result, nil
			} else {
				log.WarnC(ctx, "Operation failed with error: %v. Retry operation", err)
			}
		}
	}
}

func Retry(
	ctx context.Context,
	timeout time.Duration,
	delayBetweenAttempts time.Duration,
	operation func(context context.Context) error) error {

	_, err := RetryValue(ctx, timeout, delayBetweenAttempts, func(ctx context.Context) (any, error) {
		return nil, operation(ctx)
	})
	return err
}

func SlicesAreEqualInt32(slice1, slice2 []int32) bool {
	if len(slice1) != len(slice2) {
		return false
	}
	if len(slice1) == 0 {
		return true
	}
	for idx, val := range slice1 {
		if slice2[idx] != val {
			return false
		}
	}
	return true
}

func StringMapsAreEqual(map1, map2 map[string]*string) bool {
	if len(map1) != len(map2) {
		return false
	}
	if len(map1) == 0 {
		return true
	}
	for key, val := range map1 {
		anotherVal, found := map2[key]
		if !found {
			return false
		}
		if val == anotherVal {
			continue
		}
		if (val == nil || anotherVal == nil) || (*val != *anotherVal) {
			return false
		}
	}
	return true
}

// CheckForChangesByRequest
// This method checks the changes in originalSet by requestSet.
// It searches the parameters from requestSet in originalSet and compare their values.
// Empty requestSet means no changes in originalSet.
func CheckForChangesByRequest(originalSet, requestSet map[string]*string) bool {
	if len(requestSet) == 0 {
		return true
	}

	if len(originalSet) == 0 {
		return false
	}

	for key, val := range requestSet {
		originalVal, found := originalSet[key]
		if !found {
			return false
		}
		if val == originalVal {
			continue
		}
		if (val == nil || originalVal == nil) || (*val != *originalVal) {
			return false
		}
	}
	return true
}

func NormalizeJsonOrYamlInput(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "[") {
		// it's JSON and already in desired format
		return trimmed, nil
	} else if strings.HasPrefix(trimmed, "{") {
		// to json array
		return "[" + trimmed + "]", nil
	} else {
		// try to treat input as Yaml with triple dash delimiters
		// but imbecilic library devs always forget about that, so handle it by my tied hands

		re := regexp.MustCompile(`(?m)(^---\s*)`)
		reformatted := &strings.Builder{}
		reformatted.WriteString("[")
		needComma := false
		for _, chunk := range re.Split(raw, -1) {
			if len(strings.TrimSpace(chunk)) == 0 {
				// ignore empty chinks
				continue
			}

			if json, err := yaml.YAMLToJSON([]byte(chunk)); err == nil {
				if string(json) != "null" { // ignore commented sections
					if needComma {
						reformatted.WriteString(",")
					}
					reformatted.Write(json)
					needComma = true
				}
			} else {
				return "", err
			}
		}
		reformatted.WriteString("]")

		return reformatted.String(), nil
	}
}

func NarrowInputToOutput(input interface{}, output interface{}) error {
	m, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("error narrowing object: %w", err)
	}

	//by default all numbers for 'json' package are of type 'float'. big numbers are unmarshalled in scientific form. Using string format of numbers instead
	d := json.NewDecoder(strings.NewReader(string(m)))
	d.UseNumber()
	return d.Decode(output)
}

func NarrowInputToOutputStrict(input interface{}, output interface{}) error {
	m, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("error narrowing object: %w", err)
	}

	//by default all numbers for 'json' package are of type 'float'. big numbers are unmarshalled in scientific form. Using string format of numbers instead
	d := json.NewDecoder(strings.NewReader(string(m)))
	d.UseNumber()
	d.DisallowUnknownFields()
	return d.Decode(output)
}

func FormatterUtil(ai interface{}, state fmt.State, verb int32) {
	if verb == 's' || verb == 'q' {
		switch ai.(type) {
		case fmt.Stringer:
			fmt.Fprintf(state, ai.(fmt.Stringer).String())
			return
		default:
			// override rune and use formatting code below
			verb = 'v'
		}
	}
	switch verb {
	case 'v':
		if state.Flag('#') {
			// Emit type before
			fmt.Fprintf(state, "%T", ai)
		}
		fmt.Fprint(state, "{")
		tpe := reflect.TypeOf(ai)
		val := reflect.ValueOf(ai)
		for i := 0; i < val.NumField(); i++ {
			if state.Flag('#') || state.Flag('+') {
				fmt.Fprintf(state, "%s:", tpe.Field(i).Name)
			}

			if tpe.Field(i).Tag.Get("fmt") == "obfuscate" {
				fmt.Fprint(state, "***")
			} else {
				fmt.Fprint(state, val.Field(i))
			}

			if i < val.NumField()-1 {
				fmt.Fprint(state, " ") // field values separator
			}
		}
		fmt.Fprint(state, "}")
	default:
		fmt.Fprintf(state, "Unsupported format: ")
		fmt.Fprintf(state, string(verb))
	}
}

func ExtractName(entity interface{}) (string, error) {
	type EntityWithName struct {
		Name string `json:"name"`
	}
	var entityWithName EntityWithName
	m, err := json.Marshal(&entity)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(m, &entityWithName)
	if err != nil {
		return "", err
	}
	if entityWithName.Name != "" {
		return entityWithName.Name, nil
	} else {
		return "", fmt.Errorf("'name' field should be specified for entity: %+v: %w", entity, msg.BadRequest)
	}
}

func ExtractSourceAndDestination(binding interface{}) (string, string, error) {
	type EntityWithSourceAndDestination struct {
		Source      string `json:"source"`
		Destination string `json:"destination"`
	}
	var entity EntityWithSourceAndDestination
	m, err := json.Marshal(&binding)
	if err != nil {
		return "", "", err
	}
	err = json.Unmarshal(m, &entity)
	if err != nil {
		return "", "", err
	}

	if entity.Source != "" && entity.Destination != "" {
		return entity.Source, entity.Destination, nil
	} else {
		return "", "", fmt.Errorf("'source' and 'destination' fields should be specified for entity: %v", entity)
	}
}

func ExtractPropKey(binding interface{}) (string, error) {
	var entity map[string]interface{}
	m, err := json.Marshal(&binding)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(m, &entity)
	if err != nil {
		return "", err
	}
	if entity["properties_key"] != nil {
		return entity["properties_key"].(string), nil
	} else {
		return "", fmt.Errorf("'properties_key' field should be specified for entity: %v", entity)
	}
}

func PlatformMessageFmt(r *logging.Record, b *bytes.Buffer, color int, lvl string) (int, error) {
	timeFormat := "2006-01-02T15:04:05.000"

	//checking parameter to know, is it localdev or not. for localdev - logs with color
	var format string
	if os.Getenv("CLOUD_NAMESPACE") != "" {
		format = "[%s] [%s] [request_id=%s] [tenant_id=-] [thread=-] [class=%s] %s"
		return fmt.Fprintf(b, format,
			r.Time.Format(timeFormat),
			lvl,
			getRequestId(r.Ctx),
			logging.ConstructCallerValueByRecord(r),
			r.Message,
		)
	} else {
		format = "[%s] \x1b[%dm[%s]\x1b[0m [%s] [%s] %s"
		return fmt.Fprintf(b, format,
			r.Time.Format(timeFormat),
			color,
			lvl,
			getRequestId(r.Ctx),
			logging.ConstructCallerValueByRecord(r),
			r.Message,
		)
	}
}

func getRequestId(ctx context.Context) string {
	if ctx != nil {
		abstractRequestId := ctx.Value(xrequestid.X_REQUEST_ID_COTEXT_NAME)
		if abstractRequestId != nil {
			return abstractRequestId.(string)
		}
	}
	return "-"
}

// CancelableSleep
//
//	if timeout has been cancelled via context in the middle of sleep process,
//	then function returns false, otherwise it returns true
func CancelableSleep(ctx context.Context, amount time.Duration) bool {
	select {
	case <-time.After(amount):
		return true
	case <-ctx.Done():
		return false
	}
}

func LogError(log logging.Logger, ctx context.Context, format string, args ...any) error {
	s := fmt.Errorf(format, args...)
	log.ErrorC(ctx, s.Error())
	return s
}

func Iff[T any](condition bool, valueOnTrue T, valueOnFalse T) T {
	if condition {
		return valueOnTrue
	} else {
		return valueOnFalse
	}
}

func MatchPattern(ctx context.Context, pattern string, value any) bool {
	v, ok := value.(string)
	if !ok {
		log.ErrorC(ctx, "value '%v' must be a string", value)
		return false
	}
	match, err := path.Match(pattern, v)
	if err != nil {
		log.ErrorC(ctx, "can not match '%s' with '%s': %w", value, pattern, err)
		return false
	}
	return match
}

func RandomSubSequence(src []int32, length int) ([]int32, error) {
	if len(src) < length {
		return nil, fmt.Errorf("src size (%d) must be bigger than length (%d)", len(src), length)
	}
	r := make([]int32, length)
	perm := rand.Perm(length)
	for i, p := range perm {
		r[i] = src[p]
	}

	return r, nil
}

// difference returns the elements in `a` that aren't in `b`.
func SubstractStringsArray(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

func NewRestyClient() *resty.Client {
	client := resty.New()
	client.SetDisableWarn(true)
	return client
}

func RootCause(e error) error {
	if u := errors.Unwrap(e); u != nil {
		return RootCause(u)
	} else {
		return e
	}
}
