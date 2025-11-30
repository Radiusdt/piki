-- Vector-DSP ClickHouse Schema
-- Optimized for high-volume event storage and analytics

-- =============================================
-- CLICKS TABLE
-- =============================================

CREATE TABLE IF NOT EXISTS clicks (
    id String,
    timestamp DateTime64(3) DEFAULT now64(3),
    date Date DEFAULT toDate(timestamp),
    hour UInt8 DEFAULT toHour(timestamp),
    
    -- Campaign info
    campaign_id String,
    line_item_id String,
    creative_id String,
    advertiser_id String,
    
    -- Source info
    source_type LowCardinality(String), -- s2s, rtb
    source_id String,
    
    -- Device
    device_ifa String,
    ip String,
    user_agent String,
    
    -- Geo
    geo_country LowCardinality(String),
    geo_region LowCardinality(String),
    geo_city String,
    
    -- Device details
    device_type LowCardinality(String),
    device_os LowCardinality(String),
    device_osv String,
    device_make LowCardinality(String),
    device_model String,
    
    -- RTB specific
    bid_request_id String,
    bid_price Float64,
    win_price Float64,
    
    -- Sub IDs
    sub1 String,
    sub2 String,
    sub3 String,
    sub4 String,
    sub5 String,
    
    -- Publisher
    app_bundle String,
    publisher_id String,
    
    -- Target URL
    target_url String
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (campaign_id, source_id, date, hour, id)
TTL date + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

-- Secondary indexes
ALTER TABLE clicks ADD INDEX idx_device_ifa device_ifa TYPE bloom_filter(0.01) GRANULARITY 1;
ALTER TABLE clicks ADD INDEX idx_geo_country geo_country TYPE set(0) GRANULARITY 1;

-- =============================================
-- IMPRESSIONS TABLE
-- =============================================

CREATE TABLE IF NOT EXISTS impressions (
    id String,
    timestamp DateTime64(3) DEFAULT now64(3),
    date Date DEFAULT toDate(timestamp),
    hour UInt8 DEFAULT toHour(timestamp),
    
    -- Campaign info
    campaign_id String,
    line_item_id String,
    creative_id String,
    advertiser_id String,
    
    -- Source info
    source_type LowCardinality(String),
    source_id String,
    
    -- RTB specific
    bid_request_id String,
    bid_price Float64,
    win_price Float64,
    
    -- Device
    device_ifa String,
    ip String,
    geo_country LowCardinality(String),
    device_os LowCardinality(String),
    
    -- Publisher
    app_bundle String,
    publisher_id String
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (campaign_id, source_id, date, hour, id)
TTL date + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

-- =============================================
-- CONVERSIONS TABLE
-- =============================================

CREATE TABLE IF NOT EXISTS conversions (
    id String,
    timestamp DateTime64(3) DEFAULT now64(3),
    date Date DEFAULT toDate(timestamp),
    
    -- Click reference
    click_id String,
    click_timestamp DateTime64(3),
    
    -- Source info
    source_type LowCardinality(String),
    source_id String,
    
    -- Campaign info
    campaign_id String,
    line_item_id String,
    creative_id String,
    advertiser_id String,
    
    -- Event
    event LowCardinality(String), -- install, registration, purchase, etc.
    event_original String, -- original from MMP
    
    -- Revenue
    revenue Float64,
    revenue_currency LowCardinality(String),
    revenue_usd Float64,
    
    -- Payout
    payout Float64,
    payout_currency LowCardinality(String),
    payout_usd Float64,
    
    -- Device
    device_ifa String,
    geo_country LowCardinality(String),
    
    -- Attribution
    time_to_install Int32, -- seconds from click
    
    -- External
    external_id String
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (campaign_id, source_id, event, date, id)
TTL date + INTERVAL 365 DAY
SETTINGS index_granularity = 8192;

ALTER TABLE conversions ADD INDEX idx_click_id click_id TYPE bloom_filter(0.01) GRANULARITY 1;

-- =============================================
-- BID REQUESTS TABLE (for debugging/analysis)
-- =============================================

CREATE TABLE IF NOT EXISTS bid_requests (
    id String,
    timestamp DateTime64(3) DEFAULT now64(3),
    date Date DEFAULT toDate(timestamp),
    
    -- Source
    source_id String,
    source_name String,
    
    -- Request details
    imp_id String,
    bid_floor Float64,
    
    -- Device
    device_ifa String,
    device_os LowCardinality(String),
    device_type LowCardinality(String),
    
    -- Geo
    geo_country LowCardinality(String),
    geo_region String,
    
    -- Publisher
    app_bundle String,
    app_name String,
    publisher_id String,
    
    -- Format
    ad_format LowCardinality(String), -- banner, video, native
    width UInt16,
    height UInt16,
    
    -- Response
    bid_campaign_id String,
    bid_price Float64,
    no_bid_reason LowCardinality(String)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (source_id, date, id)
TTL date + INTERVAL 7 DAY
SETTINGS index_granularity = 8192;

-- =============================================
-- WINS TABLE
-- =============================================

CREATE TABLE IF NOT EXISTS wins (
    id String,
    timestamp DateTime64(3) DEFAULT now64(3),
    date Date DEFAULT toDate(timestamp),
    
    -- IDs
    bid_request_id String,
    imp_id String,
    
    -- Campaign
    campaign_id String,
    line_item_id String,
    creative_id String,
    
    -- Source
    source_id String,
    
    -- Pricing
    bid_price Float64,
    win_price Float64,
    
    -- Device
    device_ifa String,
    geo_country LowCardinality(String)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (campaign_id, date, id)
TTL date + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

-- =============================================
-- AGGREGATED STATS (Materialized Views)
-- =============================================

-- Hourly campaign stats
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_campaign_hourly_stats
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (campaign_id, date, hour)
AS SELECT
    campaign_id,
    toDate(timestamp) AS date,
    toHour(timestamp) AS hour,
    count() AS impressions,
    sum(win_price) AS spend
FROM impressions
GROUP BY campaign_id, date, hour;

-- Daily campaign-source stats
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_campaign_source_daily_stats
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (campaign_id, source_id, date)
AS SELECT
    campaign_id,
    source_id,
    source_type,
    toDate(timestamp) AS date,
    count() AS impressions,
    sum(win_price) AS spend
FROM impressions
GROUP BY campaign_id, source_id, source_type, date;

-- Clicks aggregation
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_clicks_daily
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (campaign_id, source_id, geo_country, date)
AS SELECT
    campaign_id,
    source_id,
    source_type,
    geo_country,
    toDate(timestamp) AS date,
    count() AS clicks
FROM clicks
GROUP BY campaign_id, source_id, source_type, geo_country, date;

-- Conversions aggregation
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_conversions_daily
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (campaign_id, source_id, event, date)
AS SELECT
    campaign_id,
    source_id,
    source_type,
    event,
    geo_country,
    toDate(timestamp) AS date,
    count() AS conversions,
    sum(revenue_usd) AS revenue,
    sum(payout_usd) AS payout
FROM conversions
GROUP BY campaign_id, source_id, source_type, event, geo_country, date;

-- =============================================
-- USEFUL QUERIES
-- =============================================

-- Campaign performance last 7 days
-- SELECT 
--     campaign_id,
--     sum(impressions) as imps,
--     sum(spend) as spend
-- FROM mv_campaign_hourly_stats
-- WHERE date >= today() - 7
-- GROUP BY campaign_id;

-- Conversion funnel by source
-- SELECT 
--     c.source_id,
--     cl.clicks,
--     cv.installs,
--     cv.revenue,
--     cv.installs / cl.clicks as cvr
-- FROM (
--     SELECT source_id, sum(clicks) as clicks 
--     FROM mv_clicks_daily 
--     WHERE date >= today() - 7 
--     GROUP BY source_id
-- ) cl
-- LEFT JOIN (
--     SELECT source_id, 
--            countIf(event = 'install') as installs,
--            sum(revenue_usd) as revenue
--     FROM conversions 
--     WHERE date >= today() - 7 
--     GROUP BY source_id
-- ) cv ON cl.source_id = cv.source_id;

-- Geo breakdown
-- SELECT 
--     geo_country,
--     count() as clicks,
--     uniq(device_ifa) as unique_devices
-- FROM clicks
-- WHERE campaign_id = 'xxx' AND date >= today() - 7
-- GROUP BY geo_country
-- ORDER BY clicks DESC
-- LIMIT 20;
