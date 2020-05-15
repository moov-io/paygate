# \ValidationApi

All URIs are relative to *http://localhost:8082*

Method | HTTP request | Description
------------- | ------------- | -------------
[**GetAccountMicroDeposits**](ValidationApi.md#GetAccountMicroDeposits) | **Get** /accounts/{accountID}/micro-deposits | Get micro-deposits for a specified accountID
[**GetMicroDeposits**](ValidationApi.md#GetMicroDeposits) | **Get** /micro-deposits/{microDepositID} | Get micro-deposit information
[**InitiateMicroDeposits**](ValidationApi.md#InitiateMicroDeposits) | **Post** /micro-deposits | Create



## GetAccountMicroDeposits

> MicroDeposits GetAccountMicroDeposits(ctx, accountID)

Get micro-deposits for a specified accountID

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**accountID** | **string**| accountID identifier from Customers service | 

### Return type

[**MicroDeposits**](MicroDeposits.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetMicroDeposits

> MicroDeposits GetMicroDeposits(ctx, microDepositID)

Get micro-deposit information

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**microDepositID** | **string**| Identifier for micro-deposits | 

### Return type

[**MicroDeposits**](MicroDeposits.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## InitiateMicroDeposits

> MicroDeposits InitiateMicroDeposits(ctx, createMicroDeposits)

Create

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**createMicroDeposits** | [**CreateMicroDeposits**](CreateMicroDeposits.md)|  | 

### Return type

[**MicroDeposits**](MicroDeposits.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

