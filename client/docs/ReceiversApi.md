# \ReceiversApi

All URIs are relative to *http://localhost:8082*

Method | HTTP request | Description
------------- | ------------- | -------------
[**AddReceivers**](ReceiversApi.md#AddReceivers) | **Post** /receivers | Create Receiver
[**DeleteReceiver**](ReceiversApi.md#DeleteReceiver) | **Delete** /receivers/{receiverID} | Delete Receiver
[**GetReceiverByID**](ReceiversApi.md#GetReceiverByID) | **Get** /receivers/{receiverID} | Get Receiver
[**GetReceivers**](ReceiversApi.md#GetReceivers) | **Get** /receivers | Get Receivers
[**UpdateReceiver**](ReceiversApi.md#UpdateReceiver) | **Patch** /receivers/{receiverID} | Update Receiver



## AddReceivers

> Receiver AddReceivers(ctx, xUserID, createReceiver, optional)

Create Receiver

Create a new Receiver under the specified x-user-id

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

Delete Receiver

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


## GetReceiverByID

> Receiver GetReceiverByID(ctx, receiverID, xUserID, optional)

Get Receiver

Get a Receiver object by it's ID for the given x-user-id

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

Get Receivers

Get all Receiver objects created for the given x-user-id

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

Update Receiver

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

