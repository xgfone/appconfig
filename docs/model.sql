CREATE TABLE `appconfig` (
    `id` INTEGER NOT NULL AUTO_INCREMENT,
    `dc` VARCHAR(32) NOT NULL COMMENT 'The name of the Data Center',
    `env` VARCHAR(32) NOT NULL COMMENT 'The name of the environment in DC',
    `app` VARCHAR(32) NOT NULL DEFAULT '' COMMENT 'The name of the application',
    `key` VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'The name of the key of app',
    `time` INTEGER NOT NULL DEFAULT 0 COMMENT 'The time to adding the record.',
    `value` TEXT DEFAULT NULL COMMENT 'The value of the key',

    PRIMARY KEY (`id`)
)


CREATE TABLE `appcallback` (
    `id` INTEGER NOT NULL AUTO_INCREMENT,
    `dc` VARCHAR(32) NOT NULL COMMENT 'The name of the Data Center',
    `env` VARCHAR(32) NOT NULL COMMENT 'The name of the environment in DC',
    `app` VARCHAR(32) NOT NULL COMMENT 'The name of the application',
    `key` VARCHAR(64) NOT NULL COMMENT 'The name of the key of app',
    `cbid` VARCHAR(64) NOT NULL COMMENT 'The id of the callback',
    `callback` VARCHAR(128) NOT NULL COMMENT 'The address of the callback, such as HTTP URL',

    PRIMARY KEY (`id`)
)


CREATE TABLE `appresult` (
    `id` INTEGER NOT NULL AUTO_INCREMENT,
    `dc` VARCHAR(32) NOT NULL COMMENT 'The name of the Data Center',
    `env` VARCHAR(32) NOT NULL COMMENT 'The name of the environment in DC',
    `app` VARCHAR(32) NOT NULL COMMENT 'The name of the application',
    `key` VARCHAR(64) NOT NULL COMMENT 'The name of the key of app',
    `cbid` VARCHAR(64) NOT NULL COMMENT 'The id of the callback',
    `callback` VARCHAR(128) NOT NULL COMMENT 'The address of the callback, such as HTTP URL',
    `result` VARCHAR(256) NOT NULL DEFAULT '' COMMENT 'The result of the callback. If successfully, it is ""; or it is the error reason.',
    `time` INTEGER NOT NULL COMMENT 'The unixstamp time when the record is inserted.',

    PRIMARY KEY (`id`)
)
