ALTER TABLE users 
ADD COLUMN reset_token VARCHAR(255),
ADD COLUMN reset_token_expires_at TIMESTAMP WITH TIME ZONE;

-- Add an index so the database can quickly look up the token
CREATE INDEX idx_users_reset_token ON users(reset_token);

ALTER TABLE users 
ADD COLUMN is_verified BOOLEAN DEFAULT false,
ADD COLUMN otp_code VARCHAR(6),
ADD COLUMN otp_expires_at TIMESTAMP WITH TIME ZONE;