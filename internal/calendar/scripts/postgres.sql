CREATE TABLE IF NOT EXISTS sys_calendar_override (
    id VARCHAR(32) NOT NULL PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT LOCALTIMESTAMP,
    created_by VARCHAR(32) NOT NULL DEFAULT 'system',
    updated_by VARCHAR(32) NOT NULL DEFAULT 'system',
    date DATE NOT NULL,
    kind VARCHAR(16) NOT NULL,
    name VARCHAR(64) NOT NULL DEFAULT '',
    CONSTRAINT uk_sys_calendar_override__date UNIQUE (date)
);
