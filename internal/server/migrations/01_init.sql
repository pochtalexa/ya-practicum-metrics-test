-- +goose Up

-- создаем таблицу -gauge- для Gauge float64 - double precision
CREATE TABLE IF NOT EXISTS gauge
(
    id      serial PRIMARY KEY,
    mname   varchar(40) UNIQUE,
    val     double precision
);

-- создаем таблицу -counter- для Counter int64 - integer
CREATE TABLE IF NOT EXISTS counter
(
    id      serial PRIMARY KEY,
    mname   varchar(40) UNIQUE,
    val     bigint
);

-- +goose Down