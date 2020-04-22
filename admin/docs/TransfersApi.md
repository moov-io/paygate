# \TransfersApi

All URIs are relative to *http://localhost:9092*

Method | HTTP request | Description
------------- | ------------- | -------------
[**UpdateTransferStatus**](TransfersApi.md#UpdateTransferStatus) | **Put** /transfers/{transferId}/status | Update Transfer status



## UpdateTransferStatus

> UpdateTransferStatus(ctx, transferId, updateTransferStatus)

Update Transfer status

Updates a Transfer status for the specified userId and transferId

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**transferId** | **string**| transferID that identifies the Transfer | 
**updateTransferStatus** | [**UpdateTransferStatus**](UpdateTransferStatus.md)|  | 

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

