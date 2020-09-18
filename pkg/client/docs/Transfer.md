# Transfer

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**TransferID** | **string** | transferID to uniquely identify this Transfer | 
**Amount** | **string** | Amount of money. USD - United States. | 
**Source** | [**Source**](Source.md) |  | 
**Destination** | [**Destination**](Destination.md) |  | 
**Description** | **string** | Brief description of the transaction, that may appear on the receiving entity’s financial statement. This field is put into the Entry Detail&#39;s DiscretionaryData.  | 
**Status** | [**TransferStatus**](TransferStatus.md) |  | 
**SameDay** | **bool** | When set to true this indicates the transfer should be processed the same day if possible. | [default to false]
**ReturnCode** | Pointer to [**ReturnCode**](ReturnCode.md) |  | [optional] 
**ProcessedAt** | Pointer to [**time.Time**](time.Time.md) |  | [optional] 
**Created** | [**time.Time**](time.Time.md) |  | 
**TraceNumbers** | **[]string** |  | 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


