sqlite3 forum.db  # To start an SQLite shell. Once you are in the SQLite shell,
.tables                   # you can check the tables in your database:
.schema users             # check the schema of your "users" table
DROP TABLE IF EXISTS users; 
.exit                     # Exit the SQLite shell:
SELECT * FROM users;
DELETE FROM users WHERE email IS NULL OR email = '';
ALTER TABLE users MODIFY email TEXT NOT NULL;

-- Create a new table with the desired schema
CREATE TABLE new_users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL,
    username TEXT,
    password TEXT
);

-- Copy data from the old table to the new one
INSERT INTO new_users (id, email, username, password)
SELECT id, email, username, password
FROM users;

-- Drop the old table
DROP TABLE users;

-- Rename the new table to the original name
ALTER TABLE new_users RENAME TO users;