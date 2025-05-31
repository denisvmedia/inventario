-- This migration should fail, causing partial failure
CREATE TABLE invalid_table (
    id SERIAL PRIMARY KEY,
    invalid_column INVALID_TYPE_THAT_DOES_NOT_EXIST
);
