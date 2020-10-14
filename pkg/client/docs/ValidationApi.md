# \ValidationApi

All URIs are relative to *http://localhost:8082*

Method | HTTP request | Description
------------- | ------------- | -------------
[**GetAccountMicroDeposits**](ValidationApi.md#GetAccountMicroDeposits) | **Get** /accounts/{accountID}/micro-deposits | Get micro-deposits for a specified accountID
[**GetMicroDeposits**](ValidationApi.md#GetMicroDeposits) | **Get** /micro-deposits/{microDepositID} | Get micro-deposit information
[**InitiateMicroDeposits**](ValidationApi.md#InitiateMicroDeposits) | **Post** /micro-deposits | Initiate micro-deposits



## GetAccountMicroDeposits

> MicroDeposits GetAccountMicroDeposits(ctx, accountID, xOrganization)

Get micro-deposits for a specified accountID

Retrieve the micro-deposits information for a specific accountID

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**accountID** | **string**| accountID identifier from Customers service | 
**xOrganization** | **string**| Value used to separate and identify models | 

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

> MicroDeposits GetMicroDeposits(ctx, microDepositID, xOrganization)

Get micro-deposit information

Retrieve the micro-deposits information for a specific microDepositID

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**microDepositID** | **string**| Identifier for micro-deposits | 
**xOrganization** | **string**| Value used to separate and identify models | 

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

> MicroDeposits InitiateMicroDeposits(ctx, xOrganization, createMicroDeposits)

Initiate micro-deposits

Start micro-deposits for a Destination to validate.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**xOrganization** | **string**| Value used to separate and identify models | 
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

