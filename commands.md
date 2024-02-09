sqlite3 forum.db  # To start an SQLite shell. Once you are in the SQLite shell,
.tables                   # you can check the tables in your database:
.schema users             # check the schema of your "users" table
DROP TABLE IF EXISTS users; 
.exit                     # Exit the SQLite shell:
SELECT * FROM users;