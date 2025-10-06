package model

import (
	"errors"
	"fmt"
)

type AggregateConfigError struct {
	Err     error
	Message string
}

func (e AggregateConfigError) Error() string {
	return fmt.Sprintf("Error during applying aggregated config, error: '%s', message: (%s)", e.Err.Error(), e.Message)
}

var ErrAggregateConfigBaseNamespace = errors.New("configurator_service: error during insert/update base namespace")
var ErrAggregateConfigParsing = errors.New("configurator_service: bad input, check correctness of your YAML")
var ErrAggregateConfigInternalMsError = errors.New("configurator_service: server error for internal config of microservice")
var ErrAggregateConfigOverallRabbitError = errors.New("configurator_service: server error for aggregate rabbit config")
var ErrAggregateConfigValidationRabbitError = errors.New("configurator_service: validation error for aggregated rabbit config")
var ErrAggregateConfigConversionError = errors.New("configurator_service: conversion error for config")
