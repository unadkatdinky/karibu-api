CREATE TABLE IF NOT EXISTS destinations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    slug VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    region VARCHAR(100) NOT NULL,
    short_description TEXT NOT NULL,
    long_description TEXT NOT NULL,
    cover_image_url TEXT NOT NULL,
    gallery_image_urls JSONB NOT NULL DEFAULT '[]'::jsonb,
    color VARCHAR(20) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_destinations_slug ON destinations(slug);

CREATE TABLE IF NOT EXISTS saved_destinations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    destination_id UUID NOT NULL REFERENCES destinations(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, destination_id)
);

CREATE INDEX idx_saved_destinations_user_id ON saved_destinations(user_id);

DROP TABLE IF EXISTS saved_places;

INSERT INTO destinations (slug, name, region, short_description, long_description, cover_image_url, gallery_image_urls, color) VALUES
('zanzibar-tanzania', 'Zanzibar', 'Tanzania',
 'Spice-scented lanes in Stone Town and white-sand beaches along the coast.',
 'Zanzibar is an archipelago off Tanzania''s coast, known for Stone Town''s narrow alleys, carved wooden doors, and centuries of Swahili, Arab, and Indian trading history. Beyond the old town, the island''s east coast offers turquoise water and powder-white beaches, while spice farms inland still grow the cloves and cinnamon that once made the island famous.',
 'https://picsum.photos/seed/zanzibar-tanzania/900/600',
 '["https://picsum.photos/seed/zanzibar-1/800/600","https://picsum.photos/seed/zanzibar-2/800/600","https://picsum.photos/seed/zanzibar-3/800/600"]'::jsonb,
 '#D4A853'),

('kilimanjaro-tanzania', 'Mount Kilimanjaro', 'Tanzania',
 'Africa''s highest peak, climbable without technical mountaineering gear.',
 'At 5,895 meters, Kilimanjaro is the tallest freestanding mountain in the world and one of the few major peaks climbable without ropes or technical experience. Routes like Machame and Marangu pass through five distinct climate zones — rainforest, moorland, alpine desert, and finally arctic summit conditions — over five to nine days.',
 'https://picsum.photos/seed/kilimanjaro-tanzania/900/600',
 '["https://picsum.photos/seed/kili-1/800/600","https://picsum.photos/seed/kili-2/800/600"]'::jsonb,
 '#2D5A3D'),

('serengeti-tanzania', 'Serengeti National Park', 'Tanzania',
 'Endless plains famous for the annual wildebeest migration.',
 'The Serengeti''s vast grasslands host the largest terrestrial mammal migration on Earth, as over a million wildebeest and zebra move in a continuous circuit chasing seasonal rains. Year-round, the park is one of the most reliable places in Africa to see lion, elephant, and cheetah in open country.',
 'https://picsum.photos/seed/serengeti-tanzania/900/600',
 '["https://picsum.photos/seed/serengeti-1/800/600","https://picsum.photos/seed/serengeti-2/800/600"]'::jsonb,
 '#C4522A'),

('volcanoes-rwanda', 'Volcanoes National Park', 'Rwanda',
 'Trek dense misty forest to spend an hour with mountain gorillas.',
 'Home to roughly a third of the world''s remaining mountain gorillas, Volcanoes National Park sits in the Virunga range along Rwanda''s northern border. Permitted treks are led by trackers and rangers, climbing through bamboo and montane forest before reaching a habituated gorilla family for a strictly limited visit.',
 'https://picsum.photos/seed/volcanoes-rwanda/900/600',
 '["https://picsum.photos/seed/rwanda-gorilla-1/800/600","https://picsum.photos/seed/rwanda-gorilla-2/800/600"]'::jsonb,
 '#1C3A2E'),

('kigali-rwanda', 'Kigali', 'Rwanda',
 'One of Africa''s cleanest capitals, with a sobering, essential history.',
 'Rwanda''s capital has been rebuilt into one of the continent''s most orderly and green cities, built across a series of hills. The Kigali Genocide Memorial is a necessary stop for understanding the country''s 1994 history, while the Kimironko Market and a growing café scene show the city''s present.',
 'https://picsum.photos/seed/kigali-rwanda/900/600',
 '["https://picsum.photos/seed/kigali-1/800/600","https://picsum.photos/seed/kigali-2/800/600"]'::jsonb,
 '#9FD4B8'),

('maasai-mara-kenya', 'Maasai Mara', 'Kenya',
 'Kenya''s premier savanna reserve, rich in wildlife year-round.',
 'The Maasai Mara is the northern extension of the Serengeti ecosystem and one of the most consistent wildlife-viewing destinations in Africa. The Mara River crossings during migration season are famous, but the reserve''s resident lion prides and big cats make it worth visiting any time of year.',
 'https://picsum.photos/seed/maasai-mara-kenya/900/600',
 '["https://picsum.photos/seed/mara-1/800/600","https://picsum.photos/seed/mara-2/800/600"]'::jsonb,
 '#D4A853'),

('lamu-kenya', 'Lamu Old Town', 'Kenya',
 'A car-free Swahili island town, unchanged for centuries.',
 'Lamu is the oldest continuously inhabited Swahili settlement in East Africa, a UNESCO World Heritage town with no cars — donkeys and dhows are still the main transport. Coral-stone buildings, carved doors, and a slow daily rhythm make it feel distinct from anywhere else on the coast.',
 'https://picsum.photos/seed/lamu-kenya/900/600',
 '["https://picsum.photos/seed/lamu-1/800/600","https://picsum.photos/seed/lamu-2/800/600"]'::jsonb,
 '#C4522A'),

('nairobi-kenya', 'Nairobi', 'Kenya',
 'The only capital city with a national park inside its limits.',
 'Nairobi National Park sits just minutes from downtown skyscrapers, home to rhino, giraffe, and lion with the city skyline visible behind them. Beyond the park, Nairobi is East Africa''s largest tech and business hub, with a fast-growing food and arts scene in neighborhoods like Kilimani and Westlands.',
 'https://picsum.photos/seed/nairobi-kenya/900/600',
 '["https://picsum.photos/seed/nairobi-1/800/600","https://picsum.photos/seed/nairobi-2/800/600"]'::jsonb,
 '#1C3A2E'),

('bwindi-uganda', 'Bwindi Impenetrable Forest', 'Uganda',
 'Uganda''s other mountain gorilla trekking destination, deep rainforest.',
 'Bwindi is one of the last remaining habitats of the mountain gorilla, an ancient rainforest so dense its name is not an exaggeration. Treks here tend to be more physically demanding than in Rwanda, through thick understory and steep terrain, but permits are generally more affordable.',
 'https://picsum.photos/seed/bwindi-uganda/900/600',
 '["https://picsum.photos/seed/bwindi-1/800/600","https://picsum.photos/seed/bwindi-2/800/600"]'::jsonb,
 '#2D5A3D'),

('jinja-uganda', 'Jinja', 'Uganda',
 'The source of the Nile, and East Africa''s adventure-sports hub.',
 'Jinja sits where the Nile River begins its journey from Lake Victoria, and has become East Africa''s center for white-water rafting, kayaking, and bungee jumping. It''s also a quieter, greener alternative base to Kampala for travelers heading toward Uganda''s national parks.',
 'https://picsum.photos/seed/jinja-uganda/900/600',
 '["https://picsum.photos/seed/jinja-1/800/600","https://picsum.photos/seed/jinja-2/800/600"]'::jsonb,
 '#D4A853'),

('ngorongoro-tanzania', 'Ngorongoro Crater', 'Tanzania',
 'A collapsed volcanic caldera holding one of Africa''s densest animal populations.',
 'Ngorongoro is the world''s largest inactive, intact volcanic caldera, its floor forming a natural enclosure that traps an unusually dense concentration of wildlife year-round, including one of Tanzania''s last black rhino populations. The crater rim also passes through Maasai grazing land, still used much as it has been for generations.',
 'https://picsum.photos/seed/ngorongoro-tanzania/900/600',
 '["https://picsum.photos/seed/ngorongoro-1/800/600","https://picsum.photos/seed/ngorongoro-2/800/600"]'::jsonb,
 '#9FD4B8'),

('nyungwe-rwanda', 'Nyungwe Forest', 'Rwanda',
 'A high-canopy walkway through one of Africa''s oldest rainforests.',
 'Nyungwe is one of the oldest and best-preserved montane rainforests in Africa, home to chimpanzees and a dozen other primate species. Its canopy walkway — a suspension bridge strung between platforms high above the forest floor — is one of the few of its kind on the continent.',
 'https://picsum.photos/seed/nyungwe-rwanda/900/600',
 '["https://picsum.photos/seed/nyungwe-1/800/600","https://picsum.photos/seed/nyungwe-2/800/600"]'::jsonb,
 '#C4522A');

CREATE TABLE destination_images (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    destination_id UUID NOT NULL REFERENCES destinations(id) ON DELETE CASCADE,
    image_url TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_destination_images_destination_id ON destination_images(destination_id);

INSERT INTO destination_images (destination_id, image_url, sort_order)
SELECT d.id, elem, ord - 1
FROM destinations d,
     jsonb_array_elements_text(d.gallery_image_urls) WITH ORDINALITY AS t(elem, ord);

ALTER TABLE destinations DROP COLUMN gallery_image_urls;