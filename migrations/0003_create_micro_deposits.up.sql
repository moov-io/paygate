CREATE TABLE `micro_deposits` (
  `micro_deposit_id` varchar(40) PRIMARY KEY NOT NULL,
  `destination_customer_id` varchar(40) NOT NULL,
  `destination_account_id` varchar(40) NOT NULL,
  `status` varchar(10) NOT NULL,
  `created_at` datetime NOT NULL,
  `deleted_at` datetime DEFAULT NULL,
  `processed_at` datetime DEFAULT NULL,
  CONSTRAINT `micro_deposits_account_id` UNIQUE (`destination_account_id`)
)
