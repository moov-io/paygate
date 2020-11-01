CREATE TABLE `transfers` (
  `transfer_id` varchar(40) PRIMARY KEY NOT NULL,
  `organization` varchar(40),
  `amount_currency` varchar(3) NOT NULL,
  `amount_value` int NOT NULL,
  `source_customer_id` varchar(40) NOT NULL,
  `source_account_id` varchar(40) NOT NULL,
  `destination_customer_id` varchar(40) NOT NULL,
  `destination_account_id` varchar(40) NOT NULL,
  `description` varchar(200) NOT NULL,
  `status` varchar(10) NOT NULL,
  `same_day` tinyint(1) NOT NULL,
  `return_code` varchar(10) DEFAULT NULL,
  `created_at` datetime NOT NULL,
  `last_updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  `remote_address` varchar(45) NOT NULL DEFAULT '',
  `processed_at` datetime DEFAULT NULL
)

