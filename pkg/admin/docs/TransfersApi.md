# \TransfersApi

All URIs are relative to *http://localhost:9092*

Method | HTTP request | Description
------------- | ------------- | -------------
[**UpdateTransferStatus**](TransfersApi.md#UpdateTransferStatus) | **Put** /transfers/{transferId}/status | Update Transfer status



## UpdateTransferStatus

> UpdateTransferStatus(ctx, transferId, xUserID, updateTransferStatus, optional)

Update Transfer status

Updates a Transfer status for the specified userId and transferId

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**transferId** | **string**| transferID that identifies the Transfer | 
**xUserID** | **string**| Unique userID set by an auth proxy or client to identify and isolate objects. | 
**updateTransferStatus** | [**UpdateTransferStatus**](UpdateTransferStatus.md)|  | 
 **optional** | ***UpdateTransferStatusOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a UpdateTransferStatusOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **xRequestID** | **optional.String**| Optional requestID allows application developer to trace requests through the systems logs | 

### Return type

 (empty response body)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

