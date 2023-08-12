CREATE TABLE `password` (
  `id` INT PRIMARY KEY AUTO_INCREMENT,
  `uid` INT,
  `cid` INT,
  `password` VARCHAR(255) COLLATE utf8mb4_general_ci,
  `url` VARCHAR(255) DEFAULT "" COLLATE utf8mb4_general_ci
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

ALTER TABLE `password` ADD COLUMN `description` VARCHAR(255);
ALTER TABLE `password` ADD COLUMN `username` VARCHAR(255) NOT NULL;
