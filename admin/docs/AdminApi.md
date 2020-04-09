# \AdminApi

All URIs are relative to *http://localhost:9092*

Method | HTTP request | Description
------------- | ------------- | -------------
[**DeleteCutoffTime**](AdminApi.md#DeleteCutoffTime) | **Delete** /configs/filetransfers/cutoff-times/{routingNumber} | Delete Cutoff
[**DeleteFTPConfig**](AdminApi.md#DeleteFTPConfig) | **Delete** /configs/filetransfers/ftp/{routingNumber} | Remove FTP Config
[**DeleteFileTransferConfig**](AdminApi.md#DeleteFileTransferConfig) | **Delete** /configs/filetransfers/{routingNumber} | Delete FileTransfer Config
[**DeleteSFTPConfig**](AdminApi.md#DeleteSFTPConfig) | **Delete** /configs/filetransfers/sftp/{routingNumber} | Remove SFTP Config
[**FlushFiles**](AdminApi.md#FlushFiles) | **Post** /files/flush | Flush files
[**FlushIncomingFiles**](AdminApi.md#FlushIncomingFiles) | **Post** /files/flush/incoming | Flush incoming files
[**FlushOutgoingFiles**](AdminApi.md#FlushOutgoingFiles) | **Post** /files/flush/outgoing | Flush outgoing files
[**GetConfigs**](AdminApi.md#GetConfigs) | **Get** /configs/filetransfers | Get FileTransfer Configs
[**GetFeatures**](AdminApi.md#GetFeatures) | **Get** /features | Get Features
[**GetMergedFile**](AdminApi.md#GetMergedFile) | **Get** /files/merged/{filename} | Get merged file
[**GetMicroDeposits**](AdminApi.md#GetMicroDeposits) | **Get** /depositories/{depositoryId}/micro-deposits | Get micro-deposits
[**GetVersion**](AdminApi.md#GetVersion) | **Get** /version | Get Version
[**ListMergedFiles**](AdminApi.md#ListMergedFiles) | **Get** /files/merged | Get merged files
[**MergeFiles**](AdminApi.md#MergeFiles) | **Post** /files/merge | Merge files
[**UpdateCutoffTime**](AdminApi.md#UpdateCutoffTime) | **Put** /configs/filetransfers/cutoff-times/{routingNumber} | Update Cutoff
[**UpdateDepositoryStatus**](AdminApi.md#UpdateDepositoryStatus) | **Put** /depositories/{depositoryId} | Update Depository Status
[**UpdateFTPConfig**](AdminApi.md#UpdateFTPConfig) | **Put** /configs/filetransfers/ftp/{routingNumber} | Update FTP Config
[**UpdateFileTransferConfig**](AdminApi.md#UpdateFileTransferConfig) | **Put** /configs/filetransfers/{routingNumber} | Update FileTransfer Config
[**UpdateSFTPConfig**](AdminApi.md#UpdateSFTPConfig) | **Put** /configs/filetransfers/sftp/{routingNumber} | Update SFTP Config
[**UpdateTransferStatus**](AdminApi.md#UpdateTransferStatus) | **Put** /transfers/{transferId}/status | Update Transfer status



## DeleteCutoffTime

> DeleteCutoffTime(ctx, routingNumber)

Delete Cutoff

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

Remove FTP Config

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

Delete FileTransfer Config

Remove a file transfer config for a given routing number

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

Remove SFTP Config

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

Flush files

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

Flush incoming files

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

Flush outgoing files

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

Get FileTransfer Configs

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

Get Features

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


## GetMergedFile

> File GetMergedFile(ctx, filename)

Get merged file

Retrieve the ACH file in JSON or plaintext for the filename

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**filename** | **string**| Filename of ACH file | 

### Return type

[**File**](File.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetMicroDeposits

> []MicroDepositAmount GetMicroDeposits(ctx, depositoryId)

Get micro-deposits

Get micro-deposits for a Depository

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**depositoryId** | **string**| Depository ID | 

### Return type

[**[]MicroDepositAmount**](MicroDepositAmount.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetVersion

> string GetVersion(ctx, )

Get Version

Show the current version of PayGate

### Required Parameters

This endpoint does not need any parameter.

### Return type

**string**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: text/plain

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## ListMergedFiles

> MergedFiles ListMergedFiles(ctx, )

Get merged files

List current files which have merged transfers and are to be uploaded

### Required Parameters

This endpoint does not need any parameter.

### Return type

[**MergedFiles**](MergedFiles.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## MergeFiles

> MergeFiles(ctx, wait)

Merge files

Merge transfers and micro-deposits into their outgoing ACH files

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


## UpdateCutoffTime

> UpdateCutoffTime(ctx, routingNumber, cutoffTime)

Update Cutoff

Update the cutoff time for a given routing number

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**routingNumber** | **string**| Routing Number | 
**cutoffTime** | [**CutoffTime**](CutoffTime.md)|  | 

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


## UpdateDepositoryStatus

> Depository UpdateDepositoryStatus(ctx, depositoryId, updateDepository)

Update Depository Status

Update Depository status for the specified depositoryId

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**depositoryId** | **string**| Depository ID | 
**updateDepository** | [**UpdateDepository**](UpdateDepository.md)|  | 

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


## UpdateFTPConfig

> UpdateFTPConfig(ctx, routingNumber, ftpConfig)

Update FTP Config

Update FTP config for a given routing number

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**routingNumber** | **string**| Routing Number | 
**ftpConfig** | [**FtpConfig**](FtpConfig.md)|  | 

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


## UpdateFileTransferConfig

> UpdateFileTransferConfig(ctx, routingNumber, fileTransferConfig)

Update FileTransfer Config

Update file transfer config for a given routing number

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**routingNumber** | **string**| Routing Number | 
**fileTransferConfig** | [**FileTransferConfig**](FileTransferConfig.md)|  | 

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


## UpdateSFTPConfig

> UpdateSFTPConfig(ctx, routingNumber, sftpConfig)

Update SFTP Config

Update SFTP config for a given routing number

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**routingNumber** | **string**| Routing Number | 
**sftpConfig** | [**SftpConfig**](SftpConfig.md)|  | 

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


## UpdateTransferStatus

> Transfer UpdateTransferStatus(ctx, transferId, updateTransferStatus)

Update Transfer status

Updates a Transfer status for the specified userId and transferId

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**transferId** | **string**| Transfer ID | 
**updateTransferStatus** | [**UpdateTransferStatus**](UpdateTransferStatus.md)|  | 

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

