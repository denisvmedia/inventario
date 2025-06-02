-- This migration should fail due to invalid SQL
CREATE TABLE invalid_table (
    id SERIAL PRIMARY KEY,
    invalid_column INVALID_TYPE_THAT_DOES_NOT_EXIST,
    another_column VARCHAR(255)
);
