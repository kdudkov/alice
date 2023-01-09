package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	gs "github.com/go-oauth2/gin-server"
	"github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/manage"
	"github.com/go-oauth2/oauth2/v4/models"
	"github.com/go-oauth2/oauth2/v4/server"
	"github.com/go-oauth2/oauth2/v4/store"
	"go.uber.org/zap"
)

var (
	gitRevision string
	gitBranch   string
)

func version() string {
	if gitBranch == "" && gitRevision == "" {
		return "unknown"
	}
	if gitBranch == "main" || gitBranch == "master" {
		return gitRevision
	}
	return gitBranch + ":" + gitRevision
}

type App struct {
	logger     *zap.SugaredLogger
	api        MahnoApi
	items      map[string]*Item
	lastChange time.Time
	mx         sync.Mutex
}

func NewApp(logger *zap.SugaredLogger) *App {
	app := &App{
		logger: logger,
		api:    NewMahnoApi(logger, "192.168.1.1:8880"),
	}

	return app
}

func (app *App) GetItem(name string) (*Item, error) {
	app.mx.Lock()
	defer app.mx.Unlock()

	if app.lastChange.Add(time.Minute).Before(time.Now()) {
		items, err := app.api.AllItems()

		if err != nil {
			return nil, err
		}

		app.items = make(map[string]*Item)
		for _, i := range items {
			app.items[i.Name] = i
		}
	}

	return app.items[name], nil
}

func (app *App) GetUnlink() gin.HandlerFunc {
	return func(ctx *gin.Context) {
	}
}

func (app *App) GetDevices() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		reqId := ctx.GetHeader("X-Request-Id")
		userId := GetUserID(ctx)
		app.logger.Infof("user %s req %s", userId, reqId)

		dev, err := app.FetchDevices()

		if err != nil {
			ctx.String(500, err.Error())
			return
		}

		v := make([]*Device, 0, len(dev))

		for _, d := range dev {
			v = append(v, d)
		}

		ctx.JSON(200, gin.H{
			"request_id": reqId,
			"payload": gin.H{
				"user_id": userId,
				"devices": v,
			},
		})
	}
}

func (app *App) GetQuery() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		reqId := ctx.GetHeader("X-Request-Id")
		userId := GetUserID(ctx)
		app.logger.Infof("user %s req %s", userId, reqId)

		devOut := make([]*Device, 0)
		dev, err := app.FetchDevices()

		if err != nil {
			ctx.String(500, err.Error())
			return
		}

		req := StateReq{}
		if err := ctx.BindJSON(&req); err != nil {
			ctx.String(500, err.Error())
			return
		}

		for _, i := range req.Devices {
			if d, ok := dev[i.Id]; ok {
				devOut = append(devOut, d)
			}
		}

		ctx.JSON(200, gin.H{
			"request_id": reqId,
			"payload": gin.H{
				"devices": devOut,
			},
		})
	}
}

func (app *App) GetAction() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		reqId := ctx.GetHeader("X-Request-Id")
		userId := GetUserID(ctx)
		app.logger.Infof("user %s req %s", userId, reqId)

		devOut := make([]*Device, 0)
		dev, err := app.FetchDevices()

		if err != nil {
			ctx.String(500, err.Error())
			return
		}

		req := ActionReq{}
		if err := ctx.BindJSON(&req); err != nil {
			ctx.String(500, err.Error())
			return
		}

		for _, i := range req.Payload.Devices {
			if len(i.Capabilities) == 0 {
				continue
			}

			if v, ok := i.Capabilities[0].GetBool("on"); ok {
				app.logger.Debugf("item %s -> %s", i.Id, OnOff(v))
				if d, ok := dev[i.Id]; ok {
					app.api.ItemCommand(i.Id, OnOff(v))
					d.Capabilities[0].SetValOk("on", v)
					devOut = append(devOut, d)
				}
			}
		}

		ctx.JSON(200, gin.H{
			"request_id": reqId,
			"payload": gin.H{
				"user_id": userId,
				"devices": devOut,
			},
		})
	}
}

func (app *App) GetHandleTokenReqest() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		app.logger.Infof("grant_type: %s", ctx.Request.FormValue("grant_type"))
		gs.HandleTokenRequest(ctx)
	}
}

func (app *App) FetchDevices() (map[string]*Device, error) {
	items, err := app.api.AllItems()

	if err != nil {
		return nil, err
	}

	dev := make(map[string]*Device)

	for _, i := range items {
		if i.Type_ == "switch" {
			sw := NewSwitch(i.Name, i.HumanName, "")

			if v, ok := i.RawValue.(bool); ok {
				sw.Capabilities[0].SetVal("on", v)
			}
			dev[sw.Id] = sw
		}
	}

	return dev, nil
}

func OnOff(a bool) string {
	if a {
		return "On"
	}
	return "Off"
}

func GetUserID(ctx *gin.Context) string {
	if ti, ok := ctx.MustGet(gs.DefaultConfig.TokenKey).(oauth2.TokenInfo); ok {
		return ti.GetUserID()
	}
	return ""
}

func (app *App) Run() {
	manager := manage.NewDefaultManager()
	manager.SetAuthorizeCodeTokenCfg(&manage.Config{
		AccessTokenExp:    0,
		RefreshTokenExp:   0,
		IsGenerateRefresh: true,
	})

	// token store
	manager.MustTokenStorage(store.NewFileTokenStore("tokens"))

	// client store
	clientStore := store.NewClientStore()
	clientStore.Set("000000", &models.Client{
		ID:     "000000",
		Secret: "999999",
		Domain: "",
	})
	manager.MapClientStorage(clientStore)

	// Initialize the oauth2 service
	gs.InitServer(manager)
	gs.SetAllowGetAccessRequest(true)
	gs.SetClientInfoHandler(server.ClientFormHandler)
	gs.SetUserAuthorizationHandler(func(w http.ResponseWriter, r *http.Request) (userID string, err error) {
		return "user", nil
	})

	gin.SetMode(gin.ReleaseMode)
	g := gin.New()
	g.Use(gin.Logger())

	g.GET("/alice/auth", gs.HandleAuthorizeRequest)

	{
		auth := g.Group("/alice/oauth2")
		auth.POST("/token", app.GetHandleTokenReqest())
	}
	{
		api := g.Group("/alice/api/v1.0")
		api.Use(gs.HandleTokenVerify())
		api.HEAD("/", func(ctx *gin.Context) {})
		api.POST("/user/unlink", app.GetUnlink())
		api.GET("/user/devices", app.GetDevices())
		api.POST("/user/devices/query", app.GetQuery())
		api.POST("/user/devices/action", app.GetAction())
	}

	g.Run(":8282")
}

func GetMap(h gin.H, name string) gin.H {
	if h == nil {
		return nil
	}
	if v, ok := h[name]; ok {
		if vv, ok := v.(map[string]any); ok {
			return vv
		}
	}
	return nil
}

func main() {
	logger, _ := zap.NewDevelopment()
	fmt.Println("version " + version())
	NewApp(logger.Sugar()).Run()
}
