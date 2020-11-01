CREATE TABLE `transfer_trace_numbers` (
  `transfer_id` varchar(40) NOT NULL,
  `trace_number` varchar(20) NOT NULL,
  CONSTRAINT `transfer_trace_numbers_idx` UNIQUE (`transfer_id`,`trace_number`)
)
