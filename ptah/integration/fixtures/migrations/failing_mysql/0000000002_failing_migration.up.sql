-- This migration intentionally fails
CREATE TABLE nonexistent_table_reference (
    id INT AUTO_INCREMENT PRIMARY KEY,
    invalid_foreign_key INT NOT NULL,
    FOREIGN KEY (invalid_foreign_key) REFERENCES nonexistent_table(id)
);
