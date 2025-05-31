-- This migration intentionally fails after some successful migrations
CREATE TABLE invalid_table (
    id INT AUTO_INCREMENT PRIMARY KEY,
    invalid_foreign_key INT NOT NULL,
    FOREIGN KEY (invalid_foreign_key) REFERENCES nonexistent_table(id)
);
