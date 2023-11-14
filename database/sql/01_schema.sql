CREATE TABLE books (
   id SERIAL PRIMARY KEY,
   title VARCHAR(255) NOT NULL,
   author VARCHAR(255) NOT NULL,
   image BYTEA,
   description TEXT,
   read BOOLEAN DEFAULT FALSE,
   added_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
   goodreads_link VARCHAR(255),
   CONSTRAINT unique_title_author UNIQUE (title, author)
);

CREATE TABLE book_likes (
    like_id SERIAL PRIMARY KEY,
    book_id INTEGER REFERENCES books(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    -- user_id TEXT REFERENCES users(user_id)
);

CREATE INDEX idx_books_title ON books USING btree (title);
CREATE INDEX idx_books_author ON books USING btree (author);
CREATE INDEX idx_books_added_on ON books USING btree (added_on);
