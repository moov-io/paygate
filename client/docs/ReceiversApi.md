# \ReceiversApi

All URIs are relative to *http://localhost:8082*

Method | HTTP request | Description
------------- | ------------- | -------------
[**AddReceivers**](ReceiversApi.md#AddReceivers) | **Post** /receivers | Create a new Receiver object
[**DeleteReceiver**](ReceiversApi.md#DeleteReceiver) | **Delete** /receivers/{receiverID} | Permanently deletes a receiver and associated depositories and transfers. It cannot be undone. Immediately cancels any active Transfers for the receiver.
[**GetDepositoriesByID**](ReceiversApi.md#GetDepositoriesByID) | **Get** /receivers/{receiverID}/depositories/{depositoryID} | Get a Depository accounts for a Receiver based on it&#39;s ID
[**GetDepositoriesByReceiverID**](ReceiversApi.md#GetDepositoriesByReceiverID) | **Get** /receivers/{receiverID}/depositories | Get a list of Depository accounts for a Receiver
[**GetReceiverByID**](ReceiversApi.md#GetReceiverByID) | **Get** /receivers/{receiverID} | Get a Receiver by ID
[**GetReceivers**](ReceiversApi.md#GetReceivers) | **Get** /receivers | Gets a list of Receivers
[**UpdateReceiver**](ReceiversApi.md#UpdateReceiver) | **Patch** /receivers/{receiverID} | Updates the specified Receiver by setting the values of the parameters passed. Any parameters not provided will be left unchanged.



## AddReceivers

> Receiver AddReceivers(ctx, xUserID, createReceiver, optional)

Create a new Receiver object

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**xUserID** | **string**| Moov User ID | 
**createReceiver** | [**CreateReceiver**](CreateReceiver.md)|  | 
 **optional** | ***AddReceiversOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a AddReceiversOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **xIdempotencyKey** | **optional.String**| Idempotent key in the header which expires after 24 hours. These strings should contain enough entropy for to not collide with each other in your requests. | 
 **xRequestID** | **optional.String**| Optional Request ID allows application developer to trace requests through the systems logs | 

### Return type

[**Receiver**](Receiver.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## DeleteReceiver

> DeleteReceiver(ctx, receiverID, xUserID, optional)

Permanently deletes a receiver and associated depositories and transfers. It cannot be undone. Immediately cancels any active Transfers for the receiver.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**receiverID** | **string**| Receiver ID | 
**xUserID** | **string**| Moov User ID | 
 **optional** | ***DeleteReceiverOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a DeleteReceiverOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **authorization** | **optional.String**| OAuth2 Bearer token | 
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


## GetDepositoriesByID

> Depository GetDepositoriesByID(ctx, receiverID, depositoryID, xUserID, optional)

Get a Depository accounts for a Receiver based on it's ID

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**receiverID** | **string**| Receiver ID | 
**depositoryID** | **string**| Depository ID | 
**xUserID** | **string**| Moov User ID | 
 **optional** | ***GetDepositoriesByIDOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetDepositoriesByIDOpts struct


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


## GetDepositoriesByReceiverID

> []Depository GetDepositoriesByReceiverID(ctx, receiverID, xUserID, optional)

Get a list of Depository accounts for a Receiver

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**receiverID** | **string**| Receiver ID | 
**xUserID** | **string**| Moov User ID | 
 **optional** | ***GetDepositoriesByReceiverIDOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetDepositoriesByReceiverIDOpts struct


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


## GetReceiverByID

> Receiver GetReceiverByID(ctx, receiverID, xUserID, optional)

Get a Receiver by ID

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**receiverID** | **string**| Receiver ID | 
**xUserID** | **string**| Moov User ID | 
 **optional** | ***GetReceiverByIDOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetReceiverByIDOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **offset** | **optional.Int32**| The number of items to skip before starting to collect the result set | [default to 0]
 **limit** | **optional.Int32**| The number of items to return | [default to 25]
 **xRequestID** | **optional.String**| Optional Request ID allows application developer to trace requests through the systems logs | 

### Return type

[**Receiver**](Receiver.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetReceivers

> []Receiver GetReceivers(ctx, xUserID, optional)

Gets a list of Receivers

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**xUserID** | **string**| Moov User ID | 
 **optional** | ***GetReceiversOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a GetReceiversOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **offset** | **optional.Int32**| The number of items to skip before starting to collect the result set | [default to 0]
 **limit** | **optional.Int32**| The number of items to return | [default to 25]
 **xRequestID** | **optional.String**| Optional Request ID allows application developer to trace requests through the systems logs | 

### Return type

[**[]Receiver**](Receiver.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## UpdateReceiver

> Receiver UpdateReceiver(ctx, receiverID, xUserID, createReceiver, optional)

Updates the specified Receiver by setting the values of the parameters passed. Any parameters not provided will be left unchanged.

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**receiverID** | **string**| Receiver ID | 
**xUserID** | **string**| Moov User ID | 
**createReceiver** | [**CreateReceiver**](CreateReceiver.md)|  | 
 **optional** | ***UpdateReceiverOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a UpdateReceiverOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **xIdempotencyKey** | **optional.String**| Idempotent key in the header which expires after 24 hours. These strings should contain enough entropy for to not collide with each other in your requests. | 
 **xRequestID** | **optional.String**| Optional Request ID allows application developer to trace requests through the systems logs | 

### Return type

[**Receiver**](Receiver.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

