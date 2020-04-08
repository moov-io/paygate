# CreateTransfer

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**TransferType** | **string** | Type of transaction being actioned against the receiving institution. Expected values are pull (debits) or push (credits). | 
**Amount** | **string** | Amount of money. USD - United States. | 
**Originator** | **string** | ID of the Originator account initiating the transfer. | 
**OriginatorDepository** | **string** | ID of the Originator Depository to be be used to override the default depository. | [optional] 
**Receiver** | **string** | ID of the Receiver account the transfer was sent to. | 
**ReceiverDepository** | **string** | ID of the Receiver Depository to be used to override the default depository | [optional] 
**Description** | **string** | Brief description of the transaction, that may appear on the receiving entity’s financial statement | 
**StandardEntryClassCode** | **string** | Standard Entry Class code will be generated based on Receiver type for CCD and PPD | [optional] 
**SameDay** | **bool** | When set to true this indicates the transfer should be processed the same day if possible. | [optional] [default to false]
**CCDDetail** | [**CcdDetail**](CCDDetail.md) |  | [optional] 
**IATDetail** | [**IatDetail**](IATDetail.md) |  | [optional] 
**PPDDetail** | [**PpdDetail**](PPDDetail.md) |  | [optional] 
**TELDetail** | [**TelDetail**](TELDetail.md) |  | [optional] 
**WEBDetail** | [**WebDetail**](WEBDetail.md) |  | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


