ALTER TABLE users 
ADD COLUMN reset_token VARCHAR(255),
ADD COLUMN reset_token_expires_at TIMESTAMP WITH TIME ZONE;

-- Add an index so the database can quickly look up the token
CREATE INDEX idx_users_reset_token ON users(reset_token);

ALTER TABLE users 
ADD COLUMN is_verified BOOLEAN DEFAULT false,
ADD COLUMN otp_code VARCHAR(6),
ADD COLUMN otp_expires_at TIMESTAMP WITH TIME ZONE;

-- ============================================
-- SAVED PLACES (first real feature table)
-- ============================================
CREATE TABLE IF NOT EXISTS saved_places (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    image_url TEXT,
    color VARCHAR(20),
    notes TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_saved_places_user_id ON saved_places(user_id);