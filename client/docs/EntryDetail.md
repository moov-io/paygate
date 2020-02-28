# EntryDetail

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**ID** | **string** | Entry Detail ID | [optional] 
**TransactionCode** | **int32** | transactionCode if the receivers account is: Credit (deposit) to checking account 22 Prenote for credit to checking account 23 Debit (withdrawal) to checking account 27 Prenote for debit to checking account 28 Credit to savings account 32 Prenote for credit to savings account 33 Debit to savings account 37 Prenote for debit to savings account 38  | 
**RDFIIdentification** | **string** | RDFI&#39;s routing number without the last digit. | 
**CheckDigit** | **string** | Last digit in RDFI routing number. | 
**DFIAccountNumber** | **string** | The receiver&#39;s bank account number you are crediting/debiting. It important to note that this is an alphanumeric field, so its space padded, no zero padded  | 
**Amount** | **int32** | Number of cents you are debiting/crediting this account | 
**IdentificationNumber** | **string** | Internal identification (alphanumeric) that you use to uniquely identify this Entry Detail Record | [optional] 
**IndividualName** | **string** | The name of the receiver, usually the name on the bank account | 
**DiscretionaryData** | **string** | DiscretionaryData allows ODFIs to include codes, of significance only to them, to enable specialized handling of the entry. There will be no standardized interpretation for the value of this field. It can either be a single two-character code, or two distinct one-character codes, according to the needs of the ODFI and/or Originator involved. This field must be returned intact for any returned entry. WEB uses the Discretionary Data Field as the Payment Type Code  | [optional] 
**AddendaRecordIndicator** | **int32** | AddendaRecordIndicator indicates the existence of an Addenda Record. A value of \&quot;1\&quot; indicates that one ore more addenda records follow, and \&quot;0\&quot; means no such record is present.  | [optional] 
**TraceNumber** | **int32** | TraceNumber assigned by the ODFI in ascending sequence, is included in each Entry Detail Record, Corporate Entry Detail Record, and addenda Record. Trace Numbers uniquely identify each entry within a batch in an ACH input file. In association with the Batch Number, transmission (File Creation) Date, and File ID Modifier, the Trace Number uniquely identifies an entry within a given file. For addenda Records, the Trace Number will be identical to the Trace Number in the associated Entry Detail Record, since the Trace Number is associated with an entry or item rather than a physical record.  | [optional] 
**Addenda02** | [**Addenda02**](Addenda02.md) |  | [optional] 
**Addenda05** | [**[]Addenda05**](Addenda05.md) | List of Addenda05 records | [optional] 
**Addenda98** | [**Addenda98**](Addenda98.md) |  | [optional] 
**Addenda99** | [**Addenda99**](Addenda99.md) |  | [optional] 
**Category** | **string** | Category defines if the entry is a Forward, Return, or NOC | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


