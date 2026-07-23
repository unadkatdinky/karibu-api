-- 1. Core Itineraries Table (The Permit Card)
CREATE TABLE itineraries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL, -- Owner of the trip
    name VARCHAR(255) NOT NULL,
    start_date DATE,
    travelers INT DEFAULT 1,
    budget DECIMAL(10, 2) DEFAULT 0.00,
    season VARCHAR(100),
    season_note TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 2. Collaborative Access Table
CREATE TABLE trip_collaborators (
    itinerary_id UUID NOT NULL REFERENCES itineraries(id) ON DELETE CASCADE,
    user_id UUID NOT NULL, -- The user being granted access
    role VARCHAR(50) DEFAULT 'editor', -- e.g., 'editor', 'viewer'
    added_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (itinerary_id, user_id)
);

-- 3. Itinerary Days Table (The Stubs)
CREATE TABLE itinerary_days (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    itinerary_id UUID NOT NULL REFERENCES itineraries(id) ON DELETE CASCADE,
    date DATE,
    region VARCHAR(50) DEFAULT 'mainland', -- 'mainland' or 'coast'
    place VARCHAR(255) NOT NULL,
    weather VARCHAR(100),
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 4. Itinerary Stops Table (The Timeline Events)
CREATE TABLE itinerary_stops (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    day_id UUID NOT NULL REFERENCES itinerary_days(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    time_label VARCHAR(50), -- e.g., '9am', 'Afternoon'
    cost DECIMAL(10, 2) DEFAULT 0.00, -- For dynamic budget calculations later
    category VARCHAR(50), -- e.g., 'Food', 'Stays', 'Transport'
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);