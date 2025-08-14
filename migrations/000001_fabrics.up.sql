CREATE TABLE IF NOT EXISTS fabrics (
  id bigint GENERATED ALWAYS AS IDENTITY,
  version int,
  code varchar(30),
  name varchar(255),
  measure_unit text,
  offer_status text,
  CONSTRAINT code UNIQUE(code)
);