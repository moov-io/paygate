# \AdminApi

All URIs are relative to *http://localhost:9092*

Method | HTTP request | Description
------------- | ------------- | -------------
[**DeleteCutoffTime**](AdminApi.md#DeleteCutoffTime) | **Delete** /configs/filetransfers/cutoff-times/{routingNumber} | Remove cutoff times for a given routing number
[**DeleteFTPConfig**](AdminApi.md#DeleteFTPConfig) | **Delete** /configs/filetransfers/ftp/{routingNumber} | Remove FTP config for a given routing number
[**DeleteFileTransferConfig**](AdminApi.md#DeleteFileTransferConfig) | **Delete** /configs/filetransfers/{routingNumber} | Remove cutoff times for a given routing number
[**DeleteSFTPConfig**](AdminApi.md#DeleteSFTPConfig) | **Delete** /configs/filetransfers/sftp/{routingNumber} | Remove SFTP config for a given routing number
[**FlushFiles**](AdminApi.md#FlushFiles) | **Post** /files/flush | Download and process all incoming and outgoing ACH files
[**FlushIncomingFiles**](AdminApi.md#FlushIncomingFiles) | **Post** /files/flush/incoming | Download and process all incoming ACH files
[**FlushOutgoingFiles**](AdminApi.md#FlushOutgoingFiles) | **Post** /files/flush/outgoing | Download and process all outgoing ACH files
[**GetConfigs**](AdminApi.md#GetConfigs) | **Get** /configs/filetransfers | Get current set of ACH file transfer configuration
[**GetFeatures**](AdminApi.md#GetFeatures) | **Get** /features | Get an object of enabled features for this PayGate instance
[**GetMicroDeposits**](AdminApi.md#GetMicroDeposits) | **Get** /depositories/{depositoryId}/micro-deposits | Get micro-deposits for a Depository
[**UpdateCutoffTime**](AdminApi.md#UpdateCutoffTime) | **Put** /configs/filetransfers/cutoff-times/{routingNumber} | Update cutoff times for a given routing number
[**UpdateDepositoryStatus**](AdminApi.md#UpdateDepositoryStatus) | **Put** /depositories/{depositoryId} | Update Depository status
[**UpdateFTPConfig**](AdminApi.md#UpdateFTPConfig) | **Put** /configs/filetransfers/ftp/{routingNumber} | Update FTP config for a given routing number
[**UpdateFileTransferConfig**](AdminApi.md#UpdateFileTransferConfig) | **Put** /configs/filetransfers/{routingNumber} | Update cutoff times for a given routing number
[**UpdateSFTPConfig**](AdminApi.md#UpdateSFTPConfig) | **Put** /configs/filetransfers/sftp/{routingNumber} | Update SFTP config for a given routing number



## DeleteCutoffTime

> DeleteCutoffTime(ctx, routingNumber)

Remove cutoff times for a given routing number

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**routingNumber** | **string**| Routing Number | 

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


## DeleteFTPConfig

> DeleteFTPConfig(ctx, routingNumber)

Remove FTP config for a given routing number

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**routingNumber** | **string**| Routing Number | 

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


## DeleteFileTransferConfig

> DeleteFileTransferConfig(ctx, routingNumber)

Remove cutoff times for a given routing number

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**routingNumber** | **string**| Routing Number | 

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


## DeleteSFTPConfig

> DeleteSFTPConfig(ctx, routingNumber)

Remove SFTP config for a given routing number

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**routingNumber** | **string**| Routing Number | 

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


## FlushFiles

> FlushFiles(ctx, wait)

Download and process all incoming and outgoing ACH files

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**wait** | **bool**| Block HTTP response until all files are processed | 

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


## FlushIncomingFiles

> FlushIncomingFiles(ctx, wait)

Download and process all incoming ACH files

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**wait** | **bool**| Block HTTP response until all files are processed | 

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


## FlushOutgoingFiles

> FlushOutgoingFiles(ctx, wait)

Download and process all outgoing ACH files

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**wait** | **bool**| Block HTTP response until all files are processed | 

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


## GetConfigs

> Configs GetConfigs(ctx, )

Get current set of ACH file transfer configuration

### Required Parameters

This endpoint does not need any parameter.

### Return type

[**Configs**](Configs.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetFeatures

> Features GetFeatures(ctx, )

Get an object of enabled features for this PayGate instance

### Required Parameters

This endpoint does not need any parameter.

### Return type

[**Features**](Features.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetMicroDeposits

> MicroDepositAmounts GetMicroDeposits(ctx, depositoryId)

Get micro-deposits for a Depository

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**depositoryId** | **string**| Depository ID | 

### Return type

[**MicroDepositAmounts**](MicroDepositAmounts.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## UpdateCutoffTime

> UpdateCutoffTime(ctx, routingNumber)

Update cutoff times for a given routing number

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**routingNumber** | **string**| Routing Number | 

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


## UpdateDepositoryStatus

> UpdateDepositoryStatus(ctx, depositoryId, updateDepository)

Update Depository status

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**depositoryId** | **string**| Depository ID | 
**updateDepository** | [**UpdateDepository**](UpdateDepository.md)|  | 

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


## UpdateFTPConfig

> UpdateFTPConfig(ctx, routingNumber)

Update FTP config for a given routing number

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**routingNumber** | **string**| Routing Number | 

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


## UpdateFileTransferConfig

> UpdateFileTransferConfig(ctx, routingNumber)

Update cutoff times for a given routing number

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**routingNumber** | **string**| Routing Number | 

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


## UpdateSFTPConfig

> UpdateSFTPConfig(ctx, routingNumber)

Update SFTP config for a given routing number

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**routingNumber** | **string**| Routing Number | 

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

