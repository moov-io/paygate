# Depository

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**ID** | **string** | Depository ID | [optional] 
**BankName** | **string** | Legal name of the financial institution. | [optional] 
**Holder** | **string** | Legal holder name on the account | 
**HolderType** | **string** | Defines the type of entity of the account holder as an *individual* or *company* | 
**Type** | **string** | Defines the account as *checking* or *savings* | 
**RoutingNumber** | **string** | The ABA routing transit number for the depository account. | 
**AccountNumber** | **string** | The account number for the depository account | 
**Status** | **string** | Defines the status of the Depository account | [optional] 
**Metadata** | **string** | Additional meta data to be used for display only | [optional] 
**ReturnCodes** | [**[]ReturnCode**](ReturnCode.md) |  | [optional] 
**Created** | [**time.Time**](time.Time.md) |  | [optional] 
**Updated** | [**time.Time**](time.Time.md) |  | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


