-- Vector-DSP Enhanced Schema (Iteration 2)
-- PostgreSQL 14+

-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================
-- ADVERTISERS
-- ============================================
CREATE TABLE IF NOT EXISTS advertisers (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    legal_name VARCHAR(255),
    tax_id VARCHAR(64),
    address TEXT,
    website VARCHAR(255),
    industry VARCHAR(64),
    status VARCHAR(32) DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_advertisers_name ON advertisers(name);
CREATE INDEX IF NOT EXISTS idx_advertisers_status ON advertisers(status);

-- ============================================
-- CAMPAIGNS
-- ============================================
CREATE TABLE IF NOT EXISTS campaigns (
    id VARCHAR(64) PRIMARY KEY,
    advertiser_id VARCHAR(64) NOT NULL REFERENCES advertisers(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'paused',
    objective VARCHAR(64),
    total_budget DECIMAL(12, 2),
    daily_budget DECIMAL(12, 2),
    start_date TIMESTAMPTZ,
    end_date TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_campaign_status CHECK (status IN ('draft', 'active', 'paused', 'ended', 'archived'))
);

CREATE INDEX IF NOT EXISTS idx_campaigns_advertiser ON campaigns(advertiser_id);
CREATE INDEX IF NOT EXISTS idx_campaigns_status ON campaigns(status);

-- ============================================
-- LINE ITEMS
-- ============================================
CREATE TABLE IF NOT EXISTS line_items (
    id VARCHAR(64) PRIMARY KEY,
    campaign_id VARCHAR(64) NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT false,
    priority INT NOT NULL DEFAULT 0,
    
    -- Bid Strategy
    bid_strategy_type VARCHAR(32) NOT NULL DEFAULT 'fixed_cpm',
    fixed_cpm DECIMAL(10, 4),
    min_cpm DECIMAL(10, 4),
    max_cpm DECIMAL(10, 4),
    target_cpa DECIMAL(10, 4),
    bid_shading DECIMAL(5, 4),
    
    -- Targeting (stored as JSONB for flexibility)
    targeting JSONB NOT NULL DEFAULT '{}',
    
    -- Pacing
    daily_budget DECIMAL(12, 2) NOT NULL,
    total_budget DECIMAL(12, 2),
    hourly_budget_cap DECIMAL(12, 2),
    start_at TIMESTAMPTZ,
    end_at TIMESTAMPTZ,
    pacing_type VARCHAR(32) DEFAULT 'even',
    freq_cap_per_user_per_day INT DEFAULT 0,
    freq_cap_per_user_per_hour INT DEFAULT 0,
    freq_cap_per_user_lifetime INT DEFAULT 0,
    qps_limit_per_source INT DEFAULT 0,
    
    -- Optimization
    optimization_goal VARCHAR(32),
    attribution_window INT DEFAULT 7,
    attribution_model VARCHAR(32) DEFAULT 'last_click',
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_line_items_campaign ON line_items(campaign_id);
CREATE INDEX IF NOT EXISTS idx_line_items_active ON line_items(is_active) WHERE is_active = true;

-- ============================================
-- CREATIVES
-- ============================================
CREATE TABLE IF NOT EXISTS creatives (
    id VARCHAR(64) PRIMARY KEY,
    advertiser_id VARCHAR(64) REFERENCES advertisers(id) ON DELETE SET NULL,
    line_item_id VARCHAR(64) REFERENCES line_items(id) ON DELETE CASCADE,
    
    name VARCHAR(255),
    format VARCHAR(32) NOT NULL DEFAULT 'banner',
    adm_template TEXT,
    width INT NOT NULL DEFAULT 0,
    height INT NOT NULL DEFAULT 0,
    adomain TEXT[],
    click_url TEXT,
    
    -- Video specific
    video_url TEXT,
    vast_tag TEXT,
    
    -- Native specific
    native_assets JSONB,
    
    -- Tracking
    impression_trackers TEXT[],
    click_trackers TEXT[],
    
    -- Audit
    audit_status VARCHAR(32) DEFAULT 'pending',
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_creative_format CHECK (format IN ('banner', 'video', 'native', 'audio'))
);

CREATE INDEX IF NOT EXISTS idx_creatives_advertiser ON creatives(advertiser_id);
CREATE INDEX IF NOT EXISTS idx_creatives_line_item ON creatives(line_item_id);
CREATE INDEX IF NOT EXISTS idx_creatives_format ON creatives(format);

-- ============================================
-- AD GROUPS
-- ============================================
CREATE TABLE IF NOT EXISTS ad_groups (
    id VARCHAR(64) PRIMARY KEY,
    campaign_id VARCHAR(64) NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    budget DECIMAL(12, 2) NOT NULL,
    start_at TIMESTAMPTZ,
    end_at TIMESTAMPTZ,
    targeting JSONB NOT NULL DEFAULT '{}',
    creative_ids TEXT[],
    is_active BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ad_groups_campaign ON ad_groups(campaign_id);

-- ============================================
-- CLICKS
-- ============================================
CREATE TABLE IF NOT EXISTS clicks (
    id VARCHAR(64) PRIMARY KEY,
    campaign_id VARCHAR(64) NOT NULL,
    line_item_id VARCHAR(64) NOT NULL,
    creative_id VARCHAR(64),
    user_id VARCHAR(255),
    target_url TEXT NOT NULL,
    ip_address INET,
    user_agent TEXT,
    geo_country VARCHAR(2),
    geo_region VARCHAR(64),
    geo_city VARCHAR(128),
    device_type INT,
    os VARCHAR(32),
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_clicks_campaign ON clicks(campaign_id);
CREATE INDEX IF NOT EXISTS idx_clicks_line_item ON clicks(line_item_id);
CREATE INDEX IF NOT EXISTS idx_clicks_timestamp ON clicks(timestamp);
CREATE INDEX IF NOT EXISTS idx_clicks_geo ON clicks(geo_country, geo_region);

-- ============================================
-- CONVERSIONS
-- ============================================
CREATE TABLE IF NOT EXISTS conversions (
    id VARCHAR(64) PRIMARY KEY,
    click_id VARCHAR(64) REFERENCES clicks(id) ON DELETE SET NULL,
    external_id VARCHAR(255),
    campaign_id VARCHAR(64),
    event_name VARCHAR(64) NOT NULL DEFAULT 'install',
    revenue DECIMAL(12, 4) DEFAULT 0,
    currency VARCHAR(3) DEFAULT 'USD',
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_conversions_click ON conversions(click_id);
CREATE INDEX IF NOT EXISTS idx_conversions_external ON conversions(external_id);
CREATE INDEX IF NOT EXISTS idx_conversions_timestamp ON conversions(timestamp);
CREATE INDEX IF NOT EXISTS idx_conversions_campaign ON conversions(campaign_id);

-- ============================================
-- IMPRESSIONS
-- ============================================
CREATE TABLE IF NOT EXISTS impressions (
    id VARCHAR(64) PRIMARY KEY,
    campaign_id VARCHAR(64) NOT NULL,
    line_item_id VARCHAR(64) NOT NULL,
    creative_id VARCHAR(64),
    bid_id VARCHAR(64),
    imp_id VARCHAR(64),
    price DECIMAL(10, 6) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    user_id VARCHAR(255),
    ip_address INET,
    geo_country VARCHAR(2),
    geo_region VARCHAR(64),
    device_type INT,
    os VARCHAR(32),
    domain VARCHAR(255),
    app_bundle VARCHAR(255),
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_impressions_campaign ON impressions(campaign_id);
CREATE INDEX IF NOT EXISTS idx_impressions_line_item ON impressions(line_item_id);
CREATE INDEX IF NOT EXISTS idx_impressions_timestamp ON impressions(timestamp);
CREATE INDEX IF NOT EXISTS idx_impressions_geo ON impressions(geo_country, geo_region);

-- ============================================
-- DAILY STATS (for faster reporting)
-- ============================================
CREATE TABLE IF NOT EXISTS daily_stats (
    id SERIAL PRIMARY KEY,
    date DATE NOT NULL,
    campaign_id VARCHAR(64) NOT NULL,
    line_item_id VARCHAR(64),
    creative_id VARCHAR(64),
    geo_country VARCHAR(2),
    device_type INT,
    os VARCHAR(32),
    
    impressions BIGINT DEFAULT 0,
    clicks BIGINT DEFAULT 0,
    conversions BIGINT DEFAULT 0,
    spend DECIMAL(12, 4) DEFAULT 0,
    revenue DECIMAL(12, 4) DEFAULT 0,
    
    UNIQUE(date, campaign_id, line_item_id, creative_id, geo_country, device_type, os)
);

CREATE INDEX IF NOT EXISTS idx_daily_stats_date ON daily_stats(date);
CREATE INDEX IF NOT EXISTS idx_daily_stats_campaign ON daily_stats(campaign_id);

-- ============================================
-- API KEYS
-- ============================================
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    advertiser_id VARCHAR(64) REFERENCES advertisers(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    permissions TEXT[] DEFAULT ARRAY['read'],
    is_active BOOLEAN NOT NULL DEFAULT true,
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_advertiser ON api_keys(advertiser_id);

-- ============================================
-- FUNCTIONS
-- ============================================

-- Auto-update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply triggers
DROP TRIGGER IF EXISTS update_advertisers_updated_at ON advertisers;
CREATE TRIGGER update_advertisers_updated_at BEFORE UPDATE ON advertisers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_campaigns_updated_at ON campaigns;
CREATE TRIGGER update_campaigns_updated_at BEFORE UPDATE ON campaigns
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_line_items_updated_at ON line_items;
CREATE TRIGGER update_line_items_updated_at BEFORE UPDATE ON line_items
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_creatives_updated_at ON creatives;
CREATE TRIGGER update_creatives_updated_at BEFORE UPDATE ON creatives
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_ad_groups_updated_at ON ad_groups;
CREATE TRIGGER update_ad_groups_updated_at BEFORE UPDATE ON ad_groups
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================
-- VIEWS
-- ============================================

-- Campaign stats view
CREATE OR REPLACE VIEW campaign_stats_view AS
SELECT 
    c.id AS campaign_id,
    c.name AS campaign_name,
    c.status,
    c.advertiser_id,
    COALESCE(ds.impressions, 0) AS impressions,
    COALESCE(ds.clicks, 0) AS clicks,
    COALESCE(ds.conversions, 0) AS conversions,
    COALESCE(ds.spend, 0) AS spend,
    COALESCE(ds.revenue, 0) AS revenue,
    CASE WHEN COALESCE(ds.impressions, 0) > 0 
        THEN ROUND(COALESCE(ds.clicks, 0)::DECIMAL / ds.impressions * 100, 4) 
        ELSE 0 
    END AS ctr,
    CASE WHEN COALESCE(ds.clicks, 0) > 0 
        THEN ROUND(COALESCE(ds.conversions, 0)::DECIMAL / ds.clicks * 100, 4) 
        ELSE 0 
    END AS cvr,
    CASE WHEN COALESCE(ds.impressions, 0) > 0 
        THEN ROUND(COALESCE(ds.spend, 0) / ds.impressions * 1000, 4) 
        ELSE 0 
    END AS ecpm
FROM campaigns c
LEFT JOIN (
    SELECT 
        campaign_id,
        SUM(impressions) AS impressions,
        SUM(clicks) AS clicks,
        SUM(conversions) AS conversions,
        SUM(spend) AS spend,
        SUM(revenue) AS revenue
    FROM daily_stats
    WHERE date >= CURRENT_DATE - INTERVAL '30 days'
    GROUP BY campaign_id
) ds ON c.id = ds.campaign_id;

-- Line item stats view
CREATE OR REPLACE VIEW line_item_stats_view AS
SELECT 
    li.id AS line_item_id,
    li.name AS line_item_name,
    li.campaign_id,
    li.is_active,
    li.daily_budget,
    COALESCE(ds.impressions, 0) AS impressions,
    COALESCE(ds.clicks, 0) AS clicks,
    COALESCE(ds.conversions, 0) AS conversions,
    COALESCE(ds.spend, 0) AS spend,
    COALESCE(ds.revenue, 0) AS revenue,
    CASE WHEN COALESCE(ds.impressions, 0) > 0 
        THEN ROUND(COALESCE(ds.clicks, 0)::DECIMAL / ds.impressions * 100, 4) 
        ELSE 0 
    END AS ctr,
    CASE WHEN COALESCE(ds.spend, 0) > 0 
        THEN ROUND(COALESCE(ds.revenue, 0) / ds.spend, 4) 
        ELSE 0 
    END AS roas
FROM line_items li
LEFT JOIN (
    SELECT 
        line_item_id,
        SUM(impressions) AS impressions,
        SUM(clicks) AS clicks,
        SUM(conversions) AS conversions,
        SUM(spend) AS spend,
        SUM(revenue) AS revenue
    FROM daily_stats
    WHERE date >= CURRENT_DATE - INTERVAL '30 days'
    GROUP BY line_item_id
) ds ON li.id = ds.line_item_id;
