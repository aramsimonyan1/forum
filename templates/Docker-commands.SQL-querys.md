# To build an image of app with (example) my-forum-app name:
  docker build -t my-forum-app .

# Run the Docker container:
  docker run -p 8080:8080 my-forum-app

# Lists running containers
  docker ps -q

# Lists stopped containers
  docker ps -a

# Stops given container
  docker stop CONTAINER ID

# Removes given container
  docker rm CONTAINER ID

# Forcefully remove all containers (stopping them if necessary)
  docker rm -f $(docker ps -a -q)

# Lists docker images 
  docker image ls

# Remove Specific Docker Images
  docker rmi IMAGE_ID

# Remove all images:
  docker rmi $(docker images -q)

# If you have dangling images (images without tags). You can remove them with:
  docker image prune



# To start an SQLite shell. Once you are in the SQLite shell,
  sqlite3 forum.db      

# Check the tables in your database
  .tables                   
  
# Check the schema of your "users" table
  .schema users             
  
# Exit the SQLite shell
  .exit                     

SELECT * FROM users;
DELETE FROM users WHERE email IS NULL OR email = '';
ALTER TABLE users MODIFY email TEXT NOT NULL;


-- Add users
INSERT INTO users (id, email, username, password) VALUES
('7a2fb2fe-9ef0-4524-9f94-0f3f613cfa2c', 'user1@example.com', 'user1', '$2a$10$YsGuZz7n6L/20zXuIfQGc.V6SRYlVMSihGpW0IPQ3M9zF1ZUYNc5G'),
('bd5d0575-0d90-4c91-be3f-c13a3a767f3b', 'user2@example.com', 'user2', '$2a$10$PwmH3wbifPyUQY6EpKr0TOWvyV3iHCWGcrrGWQ0/pTDrA.6uasdfa');

-- Add posts
INSERT INTO posts (id, title, content, category, created_at) VALUES
('post1', 'Post Title 1', 'Post Content 1', 'Category1', '2024-02-08 22:50:31'),
('post2', 'Post Title 2', 'Post Content 2', 'Category2', '2024-02-08 22:51:31');

-- Add a comment
INSERT INTO comments (id, post_id, content, created_at) VALUES
('comment1', 'post1', 'Comment Content 1', '2024-02-08 22:52:31');


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

DROP TABLE IF EXISTS users; 

-- Rename the new table to the original name
ALTER TABLE new_users RENAME TO users;

# Modify the posts table
ALTER TABLE posts
ADD COLUMN likes_count INT DEFAULT 0,
ADD COLUMN dislikes_count INT DEFAULT 0;
