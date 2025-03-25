package helper

import (
	"context"
	"encoding/json"
	"github.com/go-resty/resty/v2"
	"maas/maas-service/utils"
)

//go:generate mockgen -source=rabbit_httphelper.go -destination=mock/httphelper.go
type HttpHelper interface {
	DoRequest(ctx context.Context, method string, url string,
		user string, password string, body interface{}, allowedCodes []int, errorMessage string) (*resty.Response, error)
}

type HttpHelperImpl struct {
	httpClient *resty.Client
}

func NewHttpHelper() HttpHelper {
	return HttpHelperImpl{httpClient: utils.NewRestyClient()}
}

func (h HttpHelperImpl) DoRequest(ctx context.Context, method string, url string,
	user string, password string, body interface{}, allowedCodes []int, errorMessage string) (*resty.Response, error) {
	var s string
	if body != nil {
		bs, _ := json.Marshal(body)
		s = string(bs)
	}
	log.DebugC(ctx, "%v %v,\n\tData: %+v", method, url, s)

	response, err := h.httpClient.R().
		SetBasicAuth(user, password).
		SetBody(s).
		Execute(method, url)

	if err != nil {
		log.ErrorC(ctx, "error during sending http request: %v", err)
		return nil, err
	}

	if !intInSlice(response.RawResponse.StatusCode, allowedCodes) {
		return nil, &RabbitHttpError{
			Code:          response.RawResponse.StatusCode,
			ExpectedCodes: allowedCodes,
			Message:       errorMessage,
			Response:      response.Body(),
		}
	}
	return response, nil
}

func intInSlice(a int, list []int) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
