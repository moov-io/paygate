# Addenda02

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Id** | **string** | Client defined string used as a reference to this record. | [optional] 
**TypeCode** | **string** | 02 - NACHA regulations | [optional] 
**ReferenceInformationOne** | **string** | ReferenceInformationOne may be used for additional reference numbers, identification numbers, or codes that the merchant needs to identify the particular transaction or customer.  | [optional] 
**ReferenceInformationTwo** | **string** | ReferenceInformationTwo  may be used for additional reference numbers, identification numbers, or codes that the merchant needs to identify the particular transaction or customer.  | [optional] 
**TerminalIdentificationCode** | **string** | TerminalIdentificationCode identifies an Electronic terminal with a unique code that allows a terminal owner and/or switching network to identify the terminal at which an Entry originated.  | [optional] 
**TransactionSerialNumber** | **string** | TransactionSerialNumber is assigned by the terminal at the time the transaction is originated.  The number, with the Terminal Identification Code, serves as an audit trail for the transaction and is usually assigned in ascending sequence.  | [optional] 
**TransactionDate** | **string** | MMDD formatted timestamp identifies the date on which the transaction occurred. | [optional] 
**AuthorizationCodeOrExpireDate** | **string** | Indicates the code that a card authorization center has furnished to the merchant. | [optional] 
**TerminalLocation** | **string** | Identifies the specific location of a terminal (i.e., street names of an intersection, address, etc.) in accordance with the requirements of Regulation E. | [optional] 
**TerminalCity** | **string** | Identifies the city in which the electronic terminal is located. | [optional] 
**TerminalState** | **string** | Identifies the state in which the electronic terminal is located | [optional] 
**TraceNumber** | **string** | Entry Detail Trace Number | [optional] 

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


