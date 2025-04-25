create database beton;
use beton;
CREATE TABLE `Parameters` (
	`id_parameter` INTEGER NOT NULL AUTO_INCREMENT UNIQUE,
	`parameter_name` VARCHAR(255) NOT NULL,
	`min_threshold` FLOAT,
	`max_threshold` FLOAT,
	PRIMARY KEY(`id_parameter`)
);

CREATE TABLE `Measurement` (
	`id_measurement` INTEGER NOT NULL AUTO_INCREMENT UNIQUE,
	`time` TIME,
	`id_parameter` INTEGER,
	`value` FLOAT,
	PRIMARY KEY(`id_measurement`),
	FOREIGN KEY(`id_parameter`) REFERENCES `Parameters`(`id_parameter`)
);

CREATE TABLE `Users` (
    `id_user` INTEGER NOT NULL AUTO_INCREMENT UNIQUE,
    `login` VARCHAR(255) NOT NULL UNIQUE,
    `password` VARCHAR(255) NOT NULL UNIQUE,
    `rights` ENUM('admin', 'operator') NOT NULL,
    PRIMARY KEY(`id_user`)
);

CREATE TABLE `Reports` (
	`id_report` INTEGER NOT NULL AUTO_INCREMENT UNIQUE,
	`report_name` VARCHAR(255),
	`File` JSON,
	PRIMARY KEY(`id_report`)
);

CREATE TABLE `Logs` (
    `id_log` INTEGER NOT NULL AUTO_INCREMENT UNIQUE,
    `id_user` INTEGER NOT NULL,
    `time` DATETIME NOT NULL,
    `id_parameter` INTEGER,
    `id_report` INTEGER,
    `action_type` ENUM('parameter_update', 'login', 'logout', 'report_generated', 'threshold_alert') NOT NULL,
    `description` VARCHAR(255),
    PRIMARY KEY(`id_log`),
    FOREIGN KEY(`id_user`) REFERENCES `Users`(`id_user`),
    FOREIGN KEY(`id_parameter`) REFERENCES `Parameters`(`id_parameter`),
    FOREIGN KEY(`id_report`) REFERENCES `Reports`(`id_report`)
);

ALTER TABLE Measurement MODIFY COLUMN `time` DATETIME(3) NOT NULL;

ALTER TABLE `Reports` ADD COLUMN `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP;

INSERT INTO `Parameters` (`parameter_name`, `min_threshold`, `max_threshold`) 
VALUES 
	('speed', 960.0, 1080.0),
	('weight', 985, 1015),
	('status', 0.0, 1.0),
	('him', 10.0, 70.0),
	('humidity', 0.0, 50.0);

INSERT INTO Users (login, password, rights)
VALUES ('operator', 'operator1337', 'operator'),
       ('admin', 'admin1337', 'admin'),
       ('system', 'system', 'operator');