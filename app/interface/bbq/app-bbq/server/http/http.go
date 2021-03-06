package http

import (
	"fmt"
	"go-common/library/ecode"
	"go-common/library/net/http/blademaster/middleware/antispam"
	"net/http"

	"go-common/app/interface/bbq/app-bbq/api/http/v1"
	"go-common/app/interface/bbq/app-bbq/conf"
	"go-common/app/interface/bbq/app-bbq/service"
	xauth "go-common/app/interface/bbq/common/auth"
	chttp "go-common/app/interface/bbq/common/http"
	"go-common/library/log"
	bm "go-common/library/net/http/blademaster"
	"go-common/library/net/http/blademaster/middleware/verify"
	"go-common/library/net/trace"
)

var (
	srv              *service.Service
	vfy              *verify.Verify
	authSrv          *xauth.BannedAuth
	cfg              *conf.Config
	logger           *chttp.UILog
	likeAntiSpam     *antispam.Antispam
	relationAntiSpam *antispam.Antispam
	replyAntiSpam    *antispam.Antispam
	uploadAntiSpam   *antispam.Antispam
	reportAntiSpam   *antispam.Antispam
)

// Init init
func Init(c *conf.Config) {
	cfg = c
	initAntiSpam(c)
	logger = chttp.New(c.Infoc)
	srv = service.New(c)
	vfy = verify.New(c.Verify)
	authSrv = xauth.NewBannedAuth(c.Auth, c.MySQL)
	engine := bm.DefaultServer(c.BM)
	route(engine)
	if err := engine.Start(); err != nil {
		log.Error("bm Start error(%v)", err)
		panic(err)
	}
}

func initAntiSpam(c *conf.Config) {
	var antiConfig *antispam.Config
	var exists bool
	if antiConfig, exists = c.AntiSpam["like"]; !exists {
		panic("lose like anti_spam config")
	}
	relationAntiSpam = antispam.New(antiConfig)
	if antiConfig, exists = c.AntiSpam["relation"]; !exists {
		panic("lose relation anti_spam config")
	}
	likeAntiSpam = antispam.New(antiConfig)
	if antiConfig, exists = c.AntiSpam["reply"]; !exists {
		panic("lose reply anti_spam config")
	}
	replyAntiSpam = antispam.New(antiConfig)
	if antiConfig, exists = c.AntiSpam["upload"]; !exists {
		panic("lose upload anti_spam config")
	}
	uploadAntiSpam = antispam.New(antiConfig)

	if antiConfig, exists = c.AntiSpam["report"]; !exists {
		panic("lose report anti_spam config")
	}
	reportAntiSpam = antispam.New(antiConfig)
}

func route(e *bm.Engine) {
	e.Ping(ping)
	e.Register(register)
	g := e.Group("/bbq/app-bbq", wrapBBQ)
	{
		//????????????
		g.GET("/user/login", authSrv.User, login)
		g.POST("/user/logout", authSrv.Guest, bm.Mobile(), pushLogout)

		//????????????
		g.GET("/user/base", authSrv.User, userBase)
		// ?????????????????????????????????
		g.POST("/user/base/edit", authSrv.User, userBaseEdit)

		g.POST("/user/like/add", authSrv.User, likeAntiSpam.ServeHTTP, addUserLike)
		g.POST("/user/like/cancel", authSrv.User, likeAntiSpam.ServeHTTP, cancelUserLike)
		g.GET("/user/like/list", userLikeList)
		g.POST("/user/unlike", authSrv.User, likeAntiSpam.ServeHTTP, userUnLike)
		g.GET("/user/follow/list", authSrv.Guest, userFollowList)
		g.GET("/user/fan/list", authSrv.Guest, userFanList)
		g.GET("/user/black/list", authSrv.User, userBlackList)
		g.POST("/user/relation/modify", authSrv.User, relationAntiSpam.ServeHTTP, userRelationModify)

		g.GET("/search/hot/word", hotWord)

		// feed???????????????
		g.GET("/feed/list", authSrv.User, feedList)
		// feed???????????????
		g.GET("/feed/update_num", authSrv.User, feedUpdateNum)
		// space???????????????
		g.GET("/space/sv/list", authSrv.Guest, spaceSvList)
		// space ????????????????????????
		g.GET("/space/user/profile", authSrv.Guest, spaceUserProfile)
		// ?????????up???????????????
		g.GET("/detail/sv/list", authSrv.Guest, detailSvList)

		//????????????
		g.GET("/sv/list", authSrv.Guest, bm.Mobile(), svList)
		g.GET("/sv/playlist", authSrv.Guest, bm.Mobile(), svPlayList) // playurl??????????????????
		g.GET("/sv/detail", authSrv.Guest, svDetail)
		g.GET("/sv/stat", authSrv.Guest, bm.Mobile(), svStatistics)
		g.GET("/sv/relate", authSrv.Guest, svRelList)
		g.POST("/sv/del", authSrv.User, svDel)
		//????????????
		g.GET("/search/sv", authSrv.Guest, videoSearch)
		g.GET("/search/user", authSrv.Guest, userSearch)
		g.GET("/search/sug", authSrv.Guest, sug)
		g.GET("/search/topic", authSrv.Guest, topicSearch)
		//?????????
		g.GET("/discovery", authSrv.Guest, discoveryList)
		//???????????????
		g.GET("/topic/detail", authSrv.Guest, topicDetail)

		// ??????location
		g.GET("/location/all", authSrv.User, locationAll)
		g.GET("/location", authSrv.User, location)

		//????????????
		g.POST("/img/upload", authSrv.User, uploadAntiSpam.ServeHTTP, upload)

		// ?????????????????????
		g.GET("/share", authSrv.Guest, shareURL)
		g.GET("/share/callback", authSrv.Guest, shareCallback)

		// ?????????????????????????????????????????????????????????
		// g.GET("/invitation/download", invitationDownload)

		// App??????????????????
		g.GET("/setting", appSetting)
		g.GET("/package", appPackage)
	}
	//?????????
	r := e.Group("/bbq/app-bbq/reply", wrapBBQ, commentInit)
	{
		//????????????
		r.GET("/cursor", commentCloseRead, authSrv.Guest, commentCursor)
		r.POST("/add", commentCloseWrite, authSrv.User, phoneCheck, replyAntiSpam.ServeHTTP, commentAdd)
		r.POST("/action", commentCloseWrite, authSrv.User, likeAntiSpam.ServeHTTP, commentLike)
		r.GET("/", commentCloseRead, authSrv.Guest, commentList)
		r.GET("/reply/cursor", commentCloseRead, authSrv.Guest, commentSubCursor)
	}

	// ????????????
	report := e.Group("/bbq/app-bbq/report", wrapBBQ)
	{
		report.GET("/config", authSrv.Guest, bm.Mobile(), reportConfig)
		report.POST("/report", authSrv.Guest, bm.Mobile(), reportAntiSpam.ServeHTTP, reportReport)
	}

	// ??????????????????
	d := e.Group("/bbq/app-bbq/data", wrapBBQ)
	{
		d.GET("/collect", authSrv.Guest, bm.Mobile(), videoPlay)
	}

	// ???????????????????????????
	p := e.Group("/bbq/app-bbq/notice/center", authSrv.User, wrapBBQ)
	{
		p.GET("/num", noticeNum)
		p.GET("/overview", noticeOverview)
		p.GET("/list", noticeList)
	}

	// ????????????
	push := e.Group("/bbq/app-bbq/push", wrapBBQ, authSrv.Guest, bm.Mobile())
	{
		push.POST("/register", pushRegister)
		push.GET("/callback", pushCallback)
	}

	//??????????????????
	upload := e.Group("/bbq/app-bbq/upload/sv", authSrv.Guest)
	{
		upload.POST("/preupload", perUpload)
		upload.POST("/callback", callBack)
		upload.GET("/check", authSrv.User, uploadCheck)
		upload.POST("/homeimg", authSrv.User, homeimg)
	}
}

func commentCloseWrite(ctx *bm.Context) {
	if conf.Conf.Comment.CloseWrite {
		ctx.JSON(struct{}{}, ecode.OK)
		ctx.Abort()
	}
}
func commentCloseRead(ctx *bm.Context) {
	if conf.Conf.Comment.CloseRead {
		ctx.JSON(struct{}{}, ecode.OK)
		ctx.Abort()
	}
}

//wrapRes ??????????????????BBQ???????????????
func wrapBBQ(ctx *bm.Context) {
	chttp.WrapHeader(ctx)

	// Base params
	req := ctx.Request
	base := new(v1.Base)
	ctx.Bind(base)
	base.BUVID = req.Header.Get("Buvid")
	ctx.Set("BBQBase", base)

	// QueryID
	qid := base.QueryID
	if base.QueryID == "" {
		tracer, _ := trace.FromContext(ctx.Context)
		qid = fmt.Sprintf("%s", tracer)
	}
	ctx.Set("QueryID", qid)
}

// phoneCheck ??????????????????
func phoneCheck(ctx *bm.Context) {
	midValue, exists := ctx.Get("mid")
	if !exists {
		ctx.JSON(nil, ecode.NoLogin)
		ctx.Abort()
		return
	}
	mid := midValue.(int64)
	err := srv.PhoneCheck(ctx, mid)
	if err != nil {
		ctx.JSON(nil, err)
		ctx.Abort()
		return
	}
}

func ping(c *bm.Context) {
	if err := srv.Ping(c); err != nil {
		log.Error("ping error(%v)", err)
		c.AbortWithStatus(http.StatusServiceUnavailable)
	}
}

func register(c *bm.Context) {
	c.JSON(map[string]interface{}{}, nil)
}

func uiLog(ctx *bm.Context, action int, ext interface{}) {
	logger.Infoc(ctx, action, ext)
}
