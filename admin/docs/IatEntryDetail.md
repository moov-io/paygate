# IatEntryDetail

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**ID** | **string** | Entry Detail ID | [optional] 
**TransactionCode** | **int32** | TransactionCode if the receivers account is Credit (deposit) to checking account &#39;22&#39; Prenote for credit to checking account &#39;23&#39; Debit (withdrawal) to checking account &#39;27&#39; Prenote for debit to checking account &#39;28&#39; Credit to savings account &#39;32&#39; Prenote for credit to savings account &#39;33&#39; Debit to savings account &#39;37&#39; Prenote for debit to savings account &#39;38&#39;  | [optional] 
**RDFIIdentification** | **string** | RDFI&#39;s routing number without the last digit. | [optional] 
**CheckDigit** | **string** | Last digit in RDFI routing number. | [optional] 
**AddendaRecords** | **float32** | Number of Addenda Records | [optional] 
**Amount** | **int32** | Number of cents you are debiting/crediting this account | [optional] 
**DFIAccountNumber** | **string** | The receiver&#39;s bank account number you are crediting/debiting. It important to note that this is an alphanumeric field, so its space padded, no zero padded  | [optional] 
**OFACScreeningIndicator** | **string** | Signifies if the record has been screened against OFAC records | [optional] 
**SecondaryOFACScreeningIndicator** | **string** | Signifies if the record has been screened against OFAC records by a secondary entry | [optional] 
**AddendaRecordIndicator** | **int32** | AddendaRecordIndicator indicates the existence of an Addenda Record. A value of \&quot;1\&quot; indicates that one ore more addenda records follow, and \&quot;0\&quot; means no such record is present.  | [optional] 
**TraceNumber** | **string** | Matches the Entry Detail Trace Number of the entry being returned. | [optional] 
**Addenda10** | [**Addenda10**](Addenda10.md) |  | [optional] 
**Addenda11** | [**Addenda11**](Addenda11.md) |  | [optional] 
**Addenda12** | [**Addenda12**](Addenda12.md) |  | [optional] 
**Addenda13** | [**Addenda13**](Addenda13.md) |  | [optional] 
**Addenda14** | [**Addenda14**](Addenda14.md) |  | [optional] 
**Addenda15** | [**Addenda15**](Addenda15.md) |  | [optional] 
**Addenda16** | [**Addenda16**](Addenda16.md) |  | [optional] 
**Addenda17** | [**Addenda17**](Addenda17.md) |  | [optional] 
**Addenda18** | [**Addenda18**](Addenda18.md) |  | [optional] 
**Addenda98** | [**Addenda98**](Addenda98.md) |  | [optional] 
**Addenda99** | [**Addenda99**](Addenda99.md) |  | [optional] 
**Category** | **string** | Category defines if the entry is a Forward, Return, or NOC | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


