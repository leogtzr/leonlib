CREATE TABLE books (
   id SERIAL PRIMARY KEY,
   title VARCHAR(255) NOT NULL,
   author VARCHAR(255) NOT NULL,
   description TEXT,
   read BOOLEAN DEFAULT FALSE,
   added_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
   goodreads_link VARCHAR(255)
);

CREATE TABLE book_images (
     image_id SERIAL PRIMARY KEY,
     book_id INTEGER NOT NULL REFERENCES books(id),
     image BYTEA NOT NULL,
     added_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE users (
   user_id TEXT PRIMARY KEY,
   email TEXT NOT NULL UNIQUE,
   name TEXT,
   oauth_identifier VARCHAR NOT NULL
);

CREATE TABLE book_likes (
    like_id SERIAL PRIMARY KEY,
    book_id INTEGER REFERENCES books(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_id TEXT REFERENCES users(user_id)
);

CREATE INDEX idx_books_title ON books USING btree (title);
CREATE INDEX idx_books_author ON books USING btree (author);
CREATE INDEX idx_books_added_on ON books USING btree (added_on);
CREATE INDEX idx_book_images_book_id ON book_images USING btree (book_id);

ALTER TABLE book_likes ADD CONSTRAINT unique_book_like_per_user UNIQUE(book_id, user_id);