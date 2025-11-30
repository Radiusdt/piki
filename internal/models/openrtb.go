package models

import (
	"errors"
)

// Minimal but compatible subset of OpenRTB 2.5/2.6

type Device struct {
	Ua             string                 `json:"ua,omitempty"`
	IP             string                 `json:"ip,omitempty"`
	IPv6           string                 `json:"ipv6,omitempty"`
	OS             string                 `json:"os,omitempty"`
	OSV            string                 `json:"osv,omitempty"`
	DeviceType     int32                  `json:"devicetype,omitempty"`
	Make           string                 `json:"make,omitempty"`
	Model          string                 `json:"model,omitempty"`
	Ifa            string                 `json:"ifa,omitempty"`
	Lmt            int32                  `json:"lmt,omitempty"`
	Geo            *Geo                   `json:"geo,omitempty"`
	ConnectionType int32                  `json:"connectiontype,omitempty"`
	Carrier        string                 `json:"carrier,omitempty"`
	Language       string                 `json:"language,omitempty"`
	HwV            string                 `json:"hwv,omitempty"`
	PPI            int32                  `json:"ppi,omitempty"`
	PxRatio        float64                `json:"pxratio,omitempty"`
	JS             int32                  `json:"js,omitempty"`
	FlashVer       string                 `json:"flashver,omitempty"`
	Ext            map[string]interface{} `json:"ext,omitempty"`
}

type Geo struct {
	Lat           float64 `json:"lat,omitempty"`
	Lon           float64 `json:"lon,omitempty"`
	Type          int32   `json:"type,omitempty"`
	Accuracy      int32   `json:"accuracy,omitempty"`
	LastFix       int32   `json:"lastfix,omitempty"`
	IPService     int32   `json:"ipservice,omitempty"`
	Country       string  `json:"country,omitempty"`
	Region        string  `json:"region,omitempty"`
	RegionFIPS104 string  `json:"regionfips104,omitempty"`
	Metro         string  `json:"metro,omitempty"`
	City          string  `json:"city,omitempty"`
	ZIP           string  `json:"zip,omitempty"`
	UTCOffset     int32   `json:"utcoffset,omitempty"`
}

type User struct {
	ID         string                 `json:"id,omitempty"`
	BuyerUID   string                 `json:"buyeruid,omitempty"`
	Yob        int32                  `json:"yob,omitempty"`
	Gender     string                 `json:"gender,omitempty"`
	Keywords   string                 `json:"keywords,omitempty"`
	CustomData string                 `json:"customdata,omitempty"`
	Geo        *Geo                   `json:"geo,omitempty"`
	Data       []UserData             `json:"data,omitempty"`
	Ext        map[string]interface{} `json:"ext,omitempty"`
}

type UserData struct {
	ID      string        `json:"id,omitempty"`
	Name    string        `json:"name,omitempty"`
	Segment []UserSegment `json:"segment,omitempty"`
}

type UserSegment struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type BannerFormat struct {
	W int32 `json:"w"`
	H int32 `json:"h"`
}

type Banner struct {
	W        int32          `json:"w,omitempty"`
	H        int32          `json:"h,omitempty"`
	WMax     int32          `json:"wmax,omitempty"`
	HMax     int32          `json:"hmax,omitempty"`
	WMin     int32          `json:"wmin,omitempty"`
	HMin     int32          `json:"hmin,omitempty"`
	Format   []BannerFormat `json:"format,omitempty"`
	BType    []int32        `json:"btype,omitempty"`
	BAttr    []int32        `json:"battr,omitempty"`
	Mimes    []string       `json:"mimes,omitempty"`
	Pos      int32          `json:"pos,omitempty"`
	TopFrame int32          `json:"topframe,omitempty"`
	ExpDir   []int32        `json:"expdir,omitempty"`
	API      []int32        `json:"api,omitempty"`
	ID       string         `json:"id,omitempty"`
	VCM      int32          `json:"vcm,omitempty"`
}

// Video defines a video impression for OpenRTB.
type Video struct {
	Mimes          []string `json:"mimes,omitempty"`
	MinDuration    int32    `json:"minduration,omitempty"`
	MaxDuration    int32    `json:"maxduration,omitempty"`
	Protocols      []int32  `json:"protocols,omitempty"`
	W              int32    `json:"w,omitempty"`
	H              int32    `json:"h,omitempty"`
	StartDelay     int32    `json:"startdelay,omitempty"`
	Placement      int32    `json:"placement,omitempty"`
	Linearity      int32    `json:"linearity,omitempty"`
	Skip           int32    `json:"skip,omitempty"`
	SkipMin        int32    `json:"skipmin,omitempty"`
	SkipAfter      int32    `json:"skipafter,omitempty"`
	Sequence       int32    `json:"sequence,omitempty"`
	BAttr          []int32  `json:"battr,omitempty"`
	MaxExtended    int32    `json:"maxextended,omitempty"`
	MinBitrate     int32    `json:"minbitrate,omitempty"`
	MaxBitrate     int32    `json:"maxbitrate,omitempty"`
	BoxingAllowed  int32    `json:"boxingallowed,omitempty"`
	PlaybackMethod []int32  `json:"playbackmethod,omitempty"`
	PlaybackEnd    int32    `json:"playbackend,omitempty"`
	Delivery       []int32  `json:"delivery,omitempty"`
	Pos            int32    `json:"pos,omitempty"`
	CompanionAd    []Banner `json:"companionad,omitempty"`
	API            []int32  `json:"api,omitempty"`
	CompanionType  []int32  `json:"companiontype,omitempty"`
}

// Native defines a native impression.
type Native struct {
	Request string  `json:"request,omitempty"`
	Ver     string  `json:"ver,omitempty"`
	API     []int32 `json:"api,omitempty"`
	BAttr   []int32 `json:"battr,omitempty"`
}

// Audio defines an audio impression.
type Audio struct {
	Mimes         []string `json:"mimes,omitempty"`
	MinDuration   int32    `json:"minduration,omitempty"`
	MaxDuration   int32    `json:"maxduration,omitempty"`
	Protocols     []int32  `json:"protocols,omitempty"`
	StartDelay    int32    `json:"startdelay,omitempty"`
	Sequence      int32    `json:"sequence,omitempty"`
	BAttr         []int32  `json:"battr,omitempty"`
	MaxExtended   int32    `json:"maxextended,omitempty"`
	MinBitrate    int32    `json:"minbitrate,omitempty"`
	MaxBitrate    int32    `json:"maxbitrate,omitempty"`
	Delivery      []int32  `json:"delivery,omitempty"`
	CompanionAd   []Banner `json:"companionad,omitempty"`
	API           []int32  `json:"api,omitempty"`
	CompanionType []int32  `json:"companiontype,omitempty"`
	MaxSeq        int32    `json:"maxseq,omitempty"`
	Feed          int32    `json:"feed,omitempty"`
	Stitched      int32    `json:"stitched,omitempty"`
	NVol          int32    `json:"nvol,omitempty"`
}

type App struct {
	ID            string                 `json:"id,omitempty"`
	Name          string                 `json:"name,omitempty"`
	Bundle        string                 `json:"bundle,omitempty"`
	Domain        string                 `json:"domain,omitempty"`
	StoreURL      string                 `json:"storeurl,omitempty"`
	Cat           []string               `json:"cat,omitempty"`
	SectionCat    []string               `json:"sectioncat,omitempty"`
	PageCat       []string               `json:"pagecat,omitempty"`
	Ver           string                 `json:"ver,omitempty"`
	PrivacyPolicy int32                  `json:"privacypolicy,omitempty"`
	Paid          int32                  `json:"paid,omitempty"`
	Publisher     *Publisher             `json:"publisher,omitempty"`
	Content       *Content               `json:"content,omitempty"`
	Keywords      string                 `json:"keywords,omitempty"`
	Ext           map[string]interface{} `json:"ext,omitempty"`
}

type Site struct {
	ID            string                 `json:"id,omitempty"`
	Name          string                 `json:"name,omitempty"`
	Domain        string                 `json:"domain,omitempty"`
	Cat           []string               `json:"cat,omitempty"`
	SectionCat    []string               `json:"sectioncat,omitempty"`
	PageCat       []string               `json:"pagecat,omitempty"`
	Page          string                 `json:"page,omitempty"`
	Ref           string                 `json:"ref,omitempty"`
	Search        string                 `json:"search,omitempty"`
	Mobile        int32                  `json:"mobile,omitempty"`
	PrivacyPolicy int32                  `json:"privacypolicy,omitempty"`
	Publisher     *Publisher             `json:"publisher,omitempty"`
	Content       *Content               `json:"content,omitempty"`
	Keywords      string                 `json:"keywords,omitempty"`
	Ext           map[string]interface{} `json:"ext,omitempty"`
}

type Publisher struct {
	ID     string                 `json:"id,omitempty"`
	Name   string                 `json:"name,omitempty"`
	Cat    []string               `json:"cat,omitempty"`
	Domain string                 `json:"domain,omitempty"`
	Ext    map[string]interface{} `json:"ext,omitempty"`
}

type Content struct {
	ID                 string                 `json:"id,omitempty"`
	Episode            int32                  `json:"episode,omitempty"`
	Title              string                 `json:"title,omitempty"`
	Series             string                 `json:"series,omitempty"`
	Season             string                 `json:"season,omitempty"`
	Artist             string                 `json:"artist,omitempty"`
	Genre              string                 `json:"genre,omitempty"`
	Album              string                 `json:"album,omitempty"`
	ISRC               string                 `json:"isrc,omitempty"`
	Producer           *Producer              `json:"producer,omitempty"`
	URL                string                 `json:"url,omitempty"`
	Cat                []string               `json:"cat,omitempty"`
	ProdQ              int32                  `json:"prodq,omitempty"`
	VideoQuality       int32                  `json:"videoquality,omitempty"`
	Context            int32                  `json:"context,omitempty"`
	ContentRating      string                 `json:"contentrating,omitempty"`
	UserRating         string                 `json:"userrating,omitempty"`
	QAGMediaRating     int32                  `json:"qagmediarating,omitempty"`
	Keywords           string                 `json:"keywords,omitempty"`
	LiveStream         int32                  `json:"livestream,omitempty"`
	SourceRelationship int32                  `json:"sourcerelationship,omitempty"`
	Len                int32                  `json:"len,omitempty"`
	Language           string                 `json:"language,omitempty"`
	Embeddable         int32                  `json:"embeddable,omitempty"`
	Data               []UserData             `json:"data,omitempty"`
	Ext                map[string]interface{} `json:"ext,omitempty"`
}

type Producer struct {
	ID     string                 `json:"id,omitempty"`
	Name   string                 `json:"name,omitempty"`
	Cat    []string               `json:"cat,omitempty"`
	Domain string                 `json:"domain,omitempty"`
	Ext    map[string]interface{} `json:"ext,omitempty"`
}

type Imp struct {
	ID          string                 `json:"id"`
	Metric      []Metric               `json:"metric,omitempty"`
	Banner      *Banner                `json:"banner,omitempty"`
	Video       *Video                 `json:"video,omitempty"`
	Audio       *Audio                 `json:"audio,omitempty"`
	Native      *Native                `json:"native,omitempty"`
	PMP         *PMP                   `json:"pmp,omitempty"`
	DisplayManager    string           `json:"displaymanager,omitempty"`
	DisplayManagerVer string           `json:"displaymanagerver,omitempty"`
	Instl       int32                  `json:"instl,omitempty"`
	TagID       string                 `json:"tagid,omitempty"`
	BidFloor    float64                `json:"bidfloor,omitempty"`
	BidFloorCur string                 `json:"bidfloorcur,omitempty"`
	ClickBrowser int32                 `json:"clickbrowser,omitempty"`
	Secure      int32                  `json:"secure,omitempty"`
	IFrameBuster []string              `json:"iframebuster,omitempty"`
	Exp         int32                  `json:"exp,omitempty"`
	Ext         map[string]interface{} `json:"ext,omitempty"`
}

type Metric struct {
	Type   string  `json:"type,omitempty"`
	Value  float64 `json:"value,omitempty"`
	Vendor string  `json:"vendor,omitempty"`
}

type PMP struct {
	PrivateAuction int32  `json:"private_auction,omitempty"`
	Deals          []Deal `json:"deals,omitempty"`
}

type Deal struct {
	ID          string   `json:"id"`
	BidFloor    float64  `json:"bidfloor,omitempty"`
	BidFloorCur string   `json:"bidfloorcur,omitempty"`
	At          int32    `json:"at,omitempty"`
	WSeat       []string `json:"wseat,omitempty"`
	WADomain    []string `json:"wadomain,omitempty"`
}

type Source struct {
	FD     int32                  `json:"fd,omitempty"`
	TID    string                 `json:"tid,omitempty"`
	PChain string                 `json:"pchain,omitempty"`
	Ext    map[string]interface{} `json:"ext,omitempty"`
}

type Regs struct {
	COPPA int32                  `json:"coppa,omitempty"`
	GDPR  int32                  `json:"gdpr,omitempty"`
	Ext   map[string]interface{} `json:"ext,omitempty"`
}

type BidRequest struct {
	ID      string                 `json:"id"`
	Imp     []Imp                  `json:"imp"`
	Site    *Site                  `json:"site,omitempty"`
	App     *App                   `json:"app,omitempty"`
	Device  *Device                `json:"device,omitempty"`
	User    *User                  `json:"user,omitempty"`
	Test    int32                  `json:"test,omitempty"`
	At      int32                  `json:"at,omitempty"`
	TMax    int32                  `json:"tmax,omitempty"`
	WSeat   []string               `json:"wseat,omitempty"`
	BSeat   []string               `json:"bseat,omitempty"`
	AllImps int32                  `json:"allimps,omitempty"`
	Cur     []string               `json:"cur,omitempty"`
	WLang   []string               `json:"wlang,omitempty"`
	BCat    []string               `json:"bcat,omitempty"`
	BAdv    []string               `json:"badv,omitempty"`
	BApp    []string               `json:"bapp,omitempty"`
	Source  *Source                `json:"source,omitempty"`
	Regs    *Regs                  `json:"regs,omitempty"`
	Ext     map[string]interface{} `json:"ext,omitempty"`
}

func (br *BidRequest) Validate() error {
	if br.ID == "" {
		return errors.New("missing id")
	}
	if len(br.Imp) == 0 {
		return errors.New("missing imp")
	}
	return nil
}

// ---- RESPONSE ----

type Bid struct {
	ID       string                 `json:"id"`
	ImpID    string                 `json:"impid"`
	Price    float64                `json:"price"`
	NURL     string                 `json:"nurl,omitempty"`
	BURL     string                 `json:"burl,omitempty"`
	LURL     string                 `json:"lurl,omitempty"`
	AdM      string                 `json:"adm,omitempty"`
	AdID     string                 `json:"adid,omitempty"`
	ADomain  []string               `json:"adomain,omitempty"`
	Bundle   string                 `json:"bundle,omitempty"`
	IURL     string                 `json:"iurl,omitempty"`
	CID      string                 `json:"cid,omitempty"`
	CrID     string                 `json:"crid,omitempty"`
	CrtrID   string                 `json:"crtrid,omitempty"`
	Tactic   string                 `json:"tactic,omitempty"`
	Cat      []string               `json:"cat,omitempty"`
	Attr     []int32                `json:"attr,omitempty"`
	API      int32                  `json:"api,omitempty"`
	Protocol int32                  `json:"protocol,omitempty"`
	QAGMediaRating int32            `json:"qagmediarating,omitempty"`
	Language string                 `json:"language,omitempty"`
	DealID   string                 `json:"dealid,omitempty"`
	W        int32                  `json:"w,omitempty"`
	H        int32                  `json:"h,omitempty"`
	WRatio   int32                  `json:"wratio,omitempty"`
	HRatio   int32                  `json:"hratio,omitempty"`
	Exp      int32                  `json:"exp,omitempty"`
	Ext      map[string]interface{} `json:"ext,omitempty"`
}

type SeatBid struct {
	Seat  string                 `json:"seat,omitempty"`
	Bid   []Bid                  `json:"bid"`
	Group int32                  `json:"group,omitempty"`
	Ext   map[string]interface{} `json:"ext,omitempty"`
}

type BidResponse struct {
	ID         string                 `json:"id"`
	SeatBid    []SeatBid              `json:"seatbid"`
	BidID      string                 `json:"bidid,omitempty"`
	Cur        string                 `json:"cur,omitempty"`
	CustomData string                 `json:"customdata,omitempty"`
	NBR        int32                  `json:"nbr,omitempty"`
	Ext        map[string]interface{} `json:"ext,omitempty"`
}
