# Addenda10

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Id** | **string** | Client defined string used as a reference to this record. | [optional] 
**TypeCode** | **string** | 10 - NACHA regulations | [optional] 
**TransactionTypeCode** | **string** | Transaction Type Code Describes the type of payment ANN &#x3D; Annuity BUS &#x3D; Business/Commercial DEP &#x3D; Deposit LOA &#x3D; Loan MIS &#x3D; Miscellaneous MOR &#x3D; Mortgage PEN &#x3D; Pension RLS &#x3D; Rent/Lease REM &#x3D; Remittance2 SAL &#x3D; Salary/Payroll TAX &#x3D; Tax TEL &#x3D; Telephone-Initiated Transaction WEB &#x3D; Internet-Initiated Transaction ARC &#x3D; Accounts Receivable Entry BOC &#x3D; Back Office Conversion Entry POP &#x3D; Point of Purchase Entry RCK &#x3D; Re-presented Check Entry  | [optional] 
**ForeignPaymentAmount** | **float32** | For inbound IAT payments this field should contain the USD amount or may be blank. | [optional] 
**ForeignTraceNumber** | **string** | Trace number | [optional] 
**Name** | **string** | Receiving Company Name/Individual Name | [optional] 
**EntryDetailSequenceNumber** | **float32** | EntryDetailSequenceNumber contains the ascending sequence number section of the Entry Detail or Corporate Entry Detail Record&#39;s trace number This number is the same as the last seven digits of the trace number of the related Entry Detail Record or Corporate Entry Detail Record.  | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


