# Transfer

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**TransferID** | **string** | transferID to uniquely identify this Transfer | [optional] 
**Amount** | **string** | Amount of money. USD - United States. | [optional] 
**Source** | [**Source**](Source.md) |  | [optional] 
**Destination** | [**Destination**](Destination.md) |  | [optional] 
**Description** | **string** | Brief description of the transaction, that may appear on the receiving entity’s financial statement. This field is put into the Entry Detail&#39;s DiscretionaryData.  | [optional] 
**Status** | [**TransferStatus**](TransferStatus.md) |  | [optional] 
**SameDay** | **bool** | When set to true this indicates the transfer should be processed the same day if possible. | [optional] [default to false]
**ReturnCode** | [**ReturnCode**](ReturnCode.md) |  | [optional] 
**Created** | [**time.Time**](time.Time.md) |  | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


