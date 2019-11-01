# \TransfersApi

All URIs are relative to *http://localhost:8082*

Method | HTTP request | Description
------------- | ------------- | -------------
[**AddTransfer**](TransfersApi.md#AddTransfer) | **Post** /transfers | Create a new transfer between an Originator and a Receiver. Transfers cannot be modified. Instead delete the old and create a new transfer.
[**AddTransfers**](TransfersApi.md#AddTransfers) | **Post** /transfers/batch | Create a new list of transfer, validate, build, and process. Transfers cannot be modified.
[**DeleteTransferByID**](TransfersApi.md#DeleteTransferByID) | **Delete** /transfers/{transferID} | It is possible to recall (delete) a transfer before it has been released from the financial institution.
[**GetTransferByID**](TransfersApi.md#GetTransferByID) | **Get** /transfers/{transferID} | Get a Transfer object for the supplied ID
[**GetTransferEventsByID**](TransfersApi.md#GetTransferEventsByID) | **Get** /transfers/{transferID}/events | Get all Events associated with the Transfer object&#39;s for the supplied ID
[**GetTransferFiles**](TransfersApi.md#GetTransferFiles) | **Post** /transfers/{transferID}/files | Get the ACH files to be used in this transfer.
[**GetTransferNachaCode**](TransfersApi.md#GetTransferNachaCode) | **Post** /transfers/{transferID}/failed | Get the NACHA return code and description
[**GetTransfers**](TransfersApi.md#GetTransfers) | **Get** /transfers | A list of all Transfer objects



## AddTransfer

> Transfer AddTransfer(ctx, xUserID, createTransfer, optional)

Create a new transfer between an Originator and a Receiver. Transfers cannot be modified. Instead delete the old and create a new transfer.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**xUserID** | **string**| Moov User ID | 
**createTransfer** | [**CreateTransfer**](CreateTransfer.md)|  | 
 **optional** | ***AddTransferOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a AddTransferOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **xIdempotencyKey** | **optional.String**| Idempotent key in the header which expires after 24 hours. These strings should contain enough entropy for to not collide with each other in your requests. | 
 **xRequestID** | **optional.String**| Optional Request ID allows application developer to trace requests through the systems logs | 

### Return type

[**Transfer**](Transfer.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## AddTransfers

> []Transfer AddTransfers(ctx, xUserID, createTransfer, optional)

Create a new list of transfer, validate, build, and process. Transfers cannot be modified.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**xUserID** | **string**| Moov User ID | 
**createTransfer** | [**[]CreateTransfer**](CreateTransfer.md)|  | 
 **optional** | ***AddTransfersOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a AddTransfersOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **xIdempotencyKey** | **optional.String**| Idempotent key in the header which expires after 24 hours. These strings should contain enough entropy for to not collide with each other in your requests. | 
 **xRequestID** | **optional.String**| Optional Request ID allows application developer to trace requests through the systems logs | 

### Return type

[**[]Transfer**](Transfer.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## DeleteTransferByID

> DeleteTransferByID(ctx, transferID, xUserID, optional)

It is possible to recall (delete) a transfer before it has been released from the financial institution.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**transferID** | **string**| Transfer ID | 
**xUserID** | **string**| Moov User ID | 
 **optional** | ***DeleteTransferByIDOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a DeleteTransferByIDOpts struct


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


## GetTransferByID

> Transfer GetTransferByID(ctx, transferID, xUserID, optional)

Get a Transfer object for the supplied ID

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**transferID** | **string**| Transfer ID | 
**xUserID** | **string**| Moov User ID | 
 **optional** | ***GetTransferByIDOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetTransferByIDOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **offset** | **optional.Int32**| The number of items to skip before starting to collect the result set | [default to 0]
 **limit** | **optional.Int32**| The number of items to return | [default to 25]
 **xRequestID** | **optional.String**| Optional Request ID allows application developer to trace requests through the systems logs | 

### Return type

[**Transfer**](Transfer.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetTransferEventsByID

> []Event GetTransferEventsByID(ctx, transferID, xUserID, optional)

Get all Events associated with the Transfer object's for the supplied ID

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**transferID** | **string**| Transfer ID | 
**xUserID** | **string**| Moov User ID | 
 **optional** | ***GetTransferEventsByIDOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetTransferEventsByIDOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **offset** | **optional.Int32**| The number of items to skip before starting to collect the result set | [default to 0]
 **limit** | **optional.Int32**| The number of items to return | [default to 25]
 **xRequestID** | **optional.String**| Optional Request ID allows application developer to trace requests through the systems logs | 

### Return type

[**[]Event**](Event.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetTransferFiles

> []File GetTransferFiles(ctx, transferID, xUserID, optional)

Get the ACH files to be used in this transfer.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**transferID** | **string**| Transfer ID | 
**xUserID** | **string**| Moov User ID | 
 **optional** | ***GetTransferFilesOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetTransferFilesOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **xRequestID** | **optional.String**| Optional Request ID allows application developer to trace requests through the systems logs | 
 **xIdempotencyKey** | **optional.String**| Idempotent key in the header which expires after 24 hours. These strings should contain enough entropy for to not collide with each other in your requests. | 

### Return type

[**[]File**](File.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetTransferNachaCode

> GetTransferNachaCode(ctx, transferID, xUserID, optional)

Get the NACHA return code and description

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**transferID** | **string**| Transfer ID | 
**xUserID** | **string**| Moov User ID | 
 **optional** | ***GetTransferNachaCodeOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetTransferNachaCodeOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **xRequestID** | **optional.String**| Optional Request ID allows application developer to trace requests through the systems logs | 
 **xIdempotencyKey** | **optional.String**| Idempotent key in the header which expires after 24 hours. These strings should contain enough entropy for to not collide with each other in your requests. | 

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


## GetTransfers

> []Transfer GetTransfers(ctx, xUserID, optional)

A list of all Transfer objects

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**xUserID** | **string**| Moov User ID | 
 **optional** | ***GetTransfersOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetTransfersOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **offset** | **optional.Int32**| The number of items to skip before starting to collect the result set | [default to 0]
 **limit** | **optional.Int32**| The number of items to return | [default to 25]
 **startDate** | **optional.Time**| Filter objects created after this date. ISO-8601 format YYYY-MM-DD. Can optionally be used with endDate to specify a date range. | 
 **endDate** | **optional.Time**| Filter objects created before this date. ISO-8601 format YYYY-MM-DD. Can optionally be used with startDate to specify a date range. | 
 **xRequestID** | **optional.String**| Optional Request ID allows application developer to trace requests through the systems logs | 

### Return type

[**[]Transfer**](Transfer.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

