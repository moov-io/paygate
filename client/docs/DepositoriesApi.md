# \DepositoriesApi

All URIs are relative to *http://localhost:8082*

Method | HTTP request | Description
------------- | ------------- | -------------
[**AddDepository**](DepositoriesApi.md#AddDepository) | **Post** /depositories | Create Depository
[**ConfirmMicroDeposits**](DepositoriesApi.md#ConfirmMicroDeposits) | **Post** /depositories/{depositoryID}/micro-deposits/confirm | Confirm micro-deposits
[**DeleteDepository**](DepositoriesApi.md#DeleteDepository) | **Delete** /depositories/{depositoryID} | Delete Depository
[**GetDepositories**](DepositoriesApi.md#GetDepositories) | **Get** /depositories | List Depositories
[**GetDepositoryByID**](DepositoriesApi.md#GetDepositoryByID) | **Get** /depositories/{depositoryID} | Get Depository
[**InitiateMicroDeposits**](DepositoriesApi.md#InitiateMicroDeposits) | **Post** /depositories/{depositoryID}/micro-deposits | Initiate micro-deposits
[**UpdateDepository**](DepositoriesApi.md#UpdateDepository) | **Patch** /depositories/{depositoryID} | Update Depository



## AddDepository

> Depository AddDepository(ctx, xUserID, createDepository, optional)

Create Depository

Create a new Dpository object for the userID

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**xUserID** | **string**| Moov User ID | 
**createDepository** | [**CreateDepository**](CreateDepository.md)|  | 
 **optional** | ***AddDepositoryOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a AddDepositoryOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **xIdempotencyKey** | **optional.String**| Idempotent key in the header which expires after 24 hours. These strings should contain enough entropy for to not collide with each other in your requests. | 
 **xRequestID** | **optional.String**| Optional Request ID allows application developer to trace requests through the systems logs | 

### Return type

[**Depository**](Depository.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## ConfirmMicroDeposits

> ConfirmMicroDeposits(ctx, depositoryID, xUserID, amounts, optional)

Confirm micro-deposits

Confirm micro deposit amounts after they have been posted to the depository account

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**depositoryID** | **string**| Depository ID | 
**xUserID** | **string**| Moov User ID | 
**amounts** | [**Amounts**](Amounts.md)|  | 
 **optional** | ***ConfirmMicroDepositsOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a ConfirmMicroDepositsOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **xIdempotencyKey** | **optional.String**| Idempotent key in the header which expires after 24 hours. These strings should contain enough entropy for to not collide with each other in your requests. | 
 **xRequestID** | **optional.String**| Optional Request ID allows application developer to trace requests through the systems logs | 

### Return type

 (empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: Not defined

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## DeleteDepository

> DeleteDepository(ctx, depositoryID, xUserID, optional)

Delete Depository

Permanently deletes a depository and associated transfers. It cannot be undone. Immediately cancels any active Transfers for the depository.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**depositoryID** | **string**| Depository ID | 
**xUserID** | **string**| Moov User ID | 
 **optional** | ***DeleteDepositoryOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a DeleteDepositoryOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **xRequestID** | **optional.String**| Optional Request ID allows application developer to trace requests through the systems logs | 

### Return type

 (empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: Not defined

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetDepositories

> []Depository GetDepositories(ctx, xUserID, optional)

List Depositories

Get all Depository objects for the userID

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**xUserID** | **string**| Moov User ID | 
 **optional** | ***GetDepositoriesOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetDepositoriesOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **offset** | **optional.Int32**| The number of items to skip before starting to collect the result set | [default to 0]
 **limit** | **optional.Int32**| The number of items to return | [default to 25]
 **xRequestID** | **optional.String**| Optional Request ID allows application developer to trace requests through the systems logs | 

### Return type

[**[]Depository**](Depository.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetDepositoryByID

> Depository GetDepositoryByID(ctx, depositoryID, xUserID, optional)

Get Depository

Get a Depository object for the supplied x-user-id

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**depositoryID** | **string**| Depository ID | 
**xUserID** | **string**| Moov User ID | 
 **optional** | ***GetDepositoryByIDOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetDepositoryByIDOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **offset** | **optional.Int32**| The number of items to skip before starting to collect the result set | [default to 0]
 **limit** | **optional.Int32**| The number of items to return | [default to 25]
 **xRequestID** | **optional.String**| Optional Request ID allows application developer to trace requests through the systems logs | 

### Return type

[**Depository**](Depository.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## InitiateMicroDeposits

> InitiateMicroDeposits(ctx, depositoryID, xUserID, optional)

Initiate micro-deposits

Initiates micro deposits to be sent to the Depository institution for account validation

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**depositoryID** | **string**| Depository ID | 
**xUserID** | **string**| Moov User ID | 
 **optional** | ***InitiateMicroDepositsOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a InitiateMicroDepositsOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **xIdempotencyKey** | **optional.String**| Idempotent key in the header which expires after 24 hours. These strings should contain enough entropy for to not collide with each other in your requests. | 
 **xRequestID** | **optional.String**| Optional Request ID allows application developer to trace requests through the systems logs | 

### Return type

 (empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## UpdateDepository

> Depository UpdateDepository(ctx, depositoryID, xUserID, createDepository, optional)

Update Depository

Updates the specified Depository by setting the values of the parameters passed for the userID. Any parameters not provided will be left unchanged.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**depositoryID** | **string**| Depository ID | 
**xUserID** | **string**| Moov User ID | 
**createDepository** | [**CreateDepository**](CreateDepository.md)|  | 
 **optional** | ***UpdateDepositoryOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a UpdateDepositoryOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **xIdempotencyKey** | **optional.String**| Idempotent key in the header which expires after 24 hours. These strings should contain enough entropy for to not collide with each other in your requests. | 
 **xRequestID** | **optional.String**| Optional Request ID allows application developer to trace requests through the systems logs | 

### Return type

[**Depository**](Depository.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

