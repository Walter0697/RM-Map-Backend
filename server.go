package main

import (
	"encoding/json"
	"errors"
	"log"
	"mapmarker/backend/config"
	"mapmarker/backend/constant"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"mapmarker/backend/graph"
	"mapmarker/backend/graph/generated"
	"mapmarker/backend/initdb"
	"mapmarker/backend/middleware"
	"mapmarker/backend/seed"
	"mapmarker/backend/service"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi"
	"github.com/go-chi/cors"
	"gorm.io/gorm"
)

func main() {
	config.Init()
	config.SetupGoGuardian()
	database.Init()
	service.InitAuthStateManager()
	dbmodel.AutoMigration()
	service.StartAPIKeyCleanupWorker()
	service.StartExternalAPIAuditCleanupWorker()
	service.StartCalendarSyncWorker()

	argLength := len(os.Args[1:])
	if argLength != 0 {
		if os.Args[1] == "seed" {
			startSeed()
			return
		}
	}

	if err := initdb.InitDatabaseValue(); err != nil {
		panic(err)
	}

	if strings.EqualFold(strings.TrimSpace(os.Getenv("RELEASE_NOTES_USE_SEED_FALLBACK")), "true") {
		prepareReleaseNote()
	}

	startServer()
}

// if there is release note to add, add it
// if there isn't, don't
func prepareReleaseNote() {
	current_version := constant.AppVersion
	// notes := []string{
	// 	"[b]Bug Fixed:",
	// 	"Openrice scrapper removing and editing issue",
	// 	"[b]New Feature:",
	// 	"Country Map!",
	// 	"Canada Map for both Country map and Station map!",
	// 	"[b]Bug Fixed:",
	// 	"Fixing issue where movie item cannot be saved",
	// 	"Marker Country selection dropdown will sometimes block map view previously",
	// 	"[b]Quality Of Life",
	// 	"Sharing marker preview will also copy the link to clipboard (we cannot override the text for most social media app so this is alternative)",
	// 	"Totally different Home Page experience!!!",
	// }
	// notes := []string{
	// 	"[b]Bug Fixed:",
	// 	"Fixing issue where you cannot edit schedule",
	// }
	notes := []string{
		"[b]Bug Fixed:",
		"Fixing issue where you cannot edit marker",
	}

	log.Println("Current version " + current_version)
	exist := service.CheckReleaseNoteAdded(current_version)
	if !exist {
		log.Println("Release note not exist! Adding...")
		icon := ""
		if err := service.CreateReleaseNote(current_version, notes, &icon); err != nil {
			panic(err)
		}
		log.Println("New Release note added!")
	}
}

// start the server
func startServer() {
	port := config.Data.App.Port

	router := chi.NewRouter()
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{config.Data.App.AllowedOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))
	router.Use(middleware.Middleware())

	server := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: &graph.Resolver{}}))

	workDir, _ := os.Getwd()
	markersDir := http.Dir(filepath.Join(workDir, "uploads/markers"))
	fileServer(router, "/image/markers", markersDir)
	typesDir := http.Dir(filepath.Join(workDir, "uploads/types"))
	fileServer(router, "/image/types", typesDir)
	pinsDir := http.Dir(filepath.Join(workDir, "uploads/pins"))
	fileServer(router, "/image/pins", pinsDir)
	typePinsDir := http.Dir(filepath.Join(workDir, "uploads/typepins"))
	fileServer(router, "/image/typepins", typePinsDir)
	previewsDir := http.Dir(filepath.Join(workDir, "uploads/previews"))
	fileServer(router, "/image/previews", previewsDir)
	moviesDir := http.Dir(filepath.Join(workDir, "uploads/movies"))
	fileServer(router, "/image/movies", moviesDir)
	stationMapsDir := http.Dir(filepath.Join(workDir, "uploads/station_maps"))
	fileServer(router, "/image/station_maps", stationMapsDir)
	stationMapIconsDir := http.Dir(filepath.Join(workDir, "uploads/station_map_icons"))
	fileServer(router, "/image/station_map_icons", stationMapIconsDir)
	countriesDir := http.Dir(filepath.Join(workDir, "uploads/countries"))
	fileServer(router, "/image/countries", countriesDir)
	releaseNotesDir := http.Dir(filepath.Join(workDir, "uploads/release_notes"))
	fileServer(router, "/image/release_notes", releaseNotesDir)

	// for non dynamic asset that is required when nothing is set
	assetsDir := http.Dir(filepath.Join(workDir, "assets"))
	fileServer(router, "/image/static", assetsDir)

	if config.Data.App.Environment == "development" {
		router.Handle("/", playground.Handler("GraphQL playground", "/query"))
	}

	router.Get("/auth/mode", service.AuthModeHandler)
	router.Get("/auth/health", service.AuthHealthHandler)
	router.Get("/auth/oidc/start", service.OIDCStartHandler)
	router.Get("/auth/oidc/callback", service.OIDCCallbackHandler)
	router.Get("/calendar/google/connect", service.CalendarGoogleConnectHandler)
	router.Get("/calendar/google/callback", service.CalendarGoogleCallbackHandler)
	router.Get("/calendar/providers/status", service.CalendarProviderStatusHandler)
	router.Post("/calendar/providers/{provider}/disconnect", service.CalendarDisconnectProviderHandler)
	router.Get("/calendar/schedules/status", service.CalendarScheduleSyncStatusHandler)
	router.Post("/calendar/schedules/{id}/sync-now", service.CalendarSyncNowHandler)
	router.Post("/calendar/schedules/{id}/retry-sync", service.CalendarRetrySyncHandler)
	router.Post("/calendar/schedules/{id}/disconnect-sync", service.CalendarDisconnectSyncHandler)
	router.Route("/settings", func(r chi.Router) {
		r.Get("/preview-pin", service.SettingsGetPreviewPinHandler)
		r.Get("/reminder-time", service.SettingsGetReminderTimeHandler)
		r.Get("/history-marker-preview", service.HistoryMarkerPreviewHandler)
		r.Get("/pins", service.SettingsListPinsHandler)
		r.Get("/ios-shortcut-install-url", service.SettingsGetIOSShortcutInstallURLHandler)
		r.Get("/telegram-bot-url", service.SettingsGetTelegramBotURLHandler)
		r.Get("/release-notes", service.SettingsListReleaseNotesHandler)
		r.Put("/preview-pin", service.SettingsUpdatePreviewPinHandler)
		r.Put("/reminder-time", service.SettingsUpdateReminderTimeHandler)
	})
	router.Route("/schedules", func(r chi.Router) {
		r.Post("/travel-analysis", service.ScheduleTravelAnalysisHandler)
		r.Post("/route-preview", service.ScheduleRoutePreviewHandler)
	})
	router.Post("/exports", service.CreateOfflineExportHandler)
	router.Get("/exports/{job_id}", service.GetOfflineExportStatusHandler)
	router.Get("/exports/{job_id}/artifacts/{format}", service.GetOfflineExportArtifactHandler)
	router.Route("/auth/apikeys", func(r chi.Router) {
		r.Get("/options", service.ListAPIKeyOptionsHandler)
		r.Get("/", service.ListAPIKeysHandler)
		r.Post("/", service.CreateAPIKeyHandler)
		r.Post("/{id}/revoke", service.RevokeAPIKeyHandler)
		r.Post("/{id}/rotate", service.RotateAPIKeyHandler)
		r.Delete("/{id}", service.DeleteAPIKeyHandler)
	})
	router.Route("/integration", func(r chi.Router) {
		r.Get("/markers", service.IntegrationListMarkersHandler)
		r.Get("/markers/countries", service.IntegrationListMarkerCountriesHandler)
		r.Get("/markers/country-parts", service.IntegrationListMarkerCountryPartsHandler)
		r.Get("/markers/hashtags", service.IntegrationListMarkerHashtagsHandler)
		r.Get("/markers/nearby", service.IntegrationNearbySearchMarkersHandler)
		r.Get("/reminders/due", service.IntegrationListDueScheduleRemindersHandler)
		r.Post("/markers", service.IntegrationCreateMarkerHandler)
		r.Post("/markers/outcomes", service.IntegrationCreateMarkerOutcomeHandler)
		r.Put("/markers/{id}", service.IntegrationUpdateMarkerHandler)
		r.Delete("/markers/{id}", service.IntegrationDeleteMarkerHandler)
		r.Get("/schedules", service.IntegrationListSchedulesHandler)
		r.Post("/schedules", service.IntegrationCreateScheduleHandler)
		r.Put("/schedules/overwrite-by-date", service.IntegrationOverwriteSchedulesByDateHandler)
		r.Put("/schedules/{id}", service.IntegrationUpdateScheduleHandler)
		r.Get("/stations", service.IntegrationListStationsHandler)
		r.Put("/stations", service.IntegrationUpdateStationHandler)
		r.Get("/settings/pins", service.IntegrationListSettingsPinsHandler)
		r.Get("/settings/marker-types", service.IntegrationListSettingsMarkerTypesHandler)
		r.Get("/settings/default-pins", service.IntegrationListSettingsDefaultPinsHandler)
		r.Put("/settings/default-pins/{label}", service.IntegrationUpdateSettingsDefaultPinHandler)
		r.Get("/settings/users/{username}/preview-pin", service.IntegrationGetUserPreviewPinSelectionHandler)
		r.Put("/settings/users/{username}/preview-pin", service.IntegrationUpdateUserPreviewPinSelectionHandler)
		r.Get("/settings/users/{username}/reminder-time", service.IntegrationGetUserReminderTimeHandler)
		r.Post("/static-map-preview/geocode", service.IntegrationGeocodeStaticMapPreviewHandler)
		r.Post("/static-map-preview", service.IntegrationGenerateStaticMapPreviewHandler)
		r.Post("/exports", service.IntegrationCreateOfflineExportHandler)
		r.Get("/exports/{job_id}", service.IntegrationGetOfflineExportStatusHandler)
		r.Get("/exports/{job_id}/artifacts/{format}", service.IntegrationGetOfflineExportArtifactHandler)
		r.Post("/routes/plan", service.IntegrationPlanRouteHandler)
		r.Post("/routes/static-image", service.IntegrationGenerateRouteStaticImageHandler)
		r.Post("/calendar/google/sync-by-date", service.IntegrationCalendarGoogleSyncByDateHandler)
		r.Route("/travel-plans", func(r chi.Router) {
			r.Post("/", service.IntegrationCreateTravelPlanHandler)
			r.Get("/", service.IntegrationListTravelPlansHandler)
			r.Get("/{id}", service.IntegrationGetTravelPlanHandler)
			r.Put("/{id}", service.IntegrationUpdateTravelPlanHandler)
		})
	})
	router.Route("/travel-plans", func(r chi.Router) {
		r.Get("/", listUserTravelPlansHandler)
		r.Get("/{id}", getUserTravelPlanHandler)
	})
	router.Get("/weather/planning", service.PlanningWeatherHandler)
	router.Post("/weather/overlay-events", service.WeatherOverlayClientEventHandler)
	router.Route("/admin", func(r chi.Router) {
		r.Get("/settings/ios-shortcut-install-url", service.AdminGetIOSShortcutInstallURLHandler)
		r.Put("/settings/ios-shortcut-install-url", service.AdminUpdateIOSShortcutInstallURLHandler)
		r.Get("/settings/telegram-bot-url", service.AdminGetTelegramBotURLHandler)
		r.Put("/settings/telegram-bot-url", service.AdminUpdateTelegramBotURLHandler)
		r.Get("/settings/schedule-travel-thresholds", service.AdminGetScheduleTravelThresholdsHandler)
		r.Put("/settings/schedule-travel-thresholds", service.AdminUpdateScheduleTravelThresholdsHandler)
		r.Get("/settings/calendar-sync-durations", service.AdminGetCalendarSyncDurationsHandler)
		r.Put("/settings/calendar-sync-durations", service.AdminUpdateCalendarSyncDurationsHandler)
		r.Get("/api-usage/providers", service.AdminExternalAPIUsageProvidersHandler)
		r.Get("/api-usage/summary", service.AdminExternalAPIUsageSummaryHandler)
		r.Get("/api-usage/trends", service.AdminExternalAPIUsageTrendsHandler)
		r.Get("/cleanup/markers", service.AdminCleanupListMarkersHandler)
		r.Get("/cleanup/schedules", service.AdminCleanupListSchedulesHandler)
		r.Post("/cleanup/testing/clear", service.AdminCleanupClearTestingHandler)
		r.Delete("/cleanup/markers/{id}", service.AdminCleanupDeleteMarkerHandler)
		r.Delete("/cleanup/schedules/{id}", service.AdminCleanupDeleteScheduleHandler)
		r.Post("/cleanup/jobs", service.AdminCleanupScheduleJobHandler)
		r.Get("/service-accounts", service.AdminListServiceAccountsHandler)
		r.Post("/service-accounts", service.AdminCreateServiceAccountHandler)
		r.Put("/service-accounts/{id}", service.AdminUpdateServiceAccountHandler)
		r.Get("/train-station-maps", service.AdminListTrainStationMapsHandler)
		r.Post("/train-station-maps", service.AdminCreateTrainStationMapHandler)
		r.Put("/train-station-maps/{map_name}", service.AdminUpdateTrainStationMapHandler)
		r.Delete("/train-station-maps/{map_name}", service.AdminDeleteTrainStationMapHandler)
		r.Get("/stations", service.AdminListStationsHandler)
		r.Put("/stations", service.AdminUpsertStationHandler)
		r.Put("/stations/lines", service.AdminUpdateStationLinesHandler)
		r.Get("/station-lines", service.AdminListStationLineCatalogHandler)
		r.Put("/station-lines", service.AdminSaveStationLineCatalogHandler)
		r.Get("/stations/export/{map_name}", service.AdminExportStationJSONHandler)
		r.Post("/stations/import/{map_name}", service.AdminImportStationJSONHandler)
		r.Post("/station-map-icons/{map_name}", service.AdminUploadStationMapIconHandler)
		r.Get("/station-maps/{map_name}", service.AdminGetStationMapAssetHandler)
		r.Post("/station-maps/{map_name}", service.AdminUploadStationMapAssetHandler)
		r.Get("/release-notes", service.AdminListReleaseNotesHandler)
		r.Post("/release-notes", service.AdminCreateReleaseNoteHandler)
		r.Get("/release-notes/{id}", service.AdminGetReleaseNoteHandler)
		r.Put("/release-notes/{id}", service.AdminUpdateReleaseNoteHandler)
		r.Delete("/release-notes/{id}", service.AdminDeleteReleaseNoteHandler)
		r.Post("/release-notes/{id}/publish", service.AdminPublishReleaseNoteHandler)
		r.Post("/release-notes/{id}/unpublish", service.AdminUnpublishReleaseNoteHandler)
		r.Post("/release-notes/images", service.AdminUploadReleaseNoteImageHandler)
		r.Get("/pin-groups", service.AdminListPinGroupsHandler)
		r.Post("/pin-groups", service.AdminCreatePinGroupHandler)
		r.Put("/pin-groups/{id}", service.AdminUpdatePinGroupHandler)
		r.Delete("/pin-groups/{id}", service.AdminDeletePinGroupHandler)
		r.Get("/pins/{id}/groups", service.AdminGetPinGroupAssignmentsHandler)
		r.Put("/pins/{id}/groups", service.AdminUpdatePinGroupAssignmentsHandler)
	})
	router.Get("/station-maps/{map_name}", service.GetStationMapAssetHandler)
	router.Handle("/query", server)

	if config.Data.App.Environment == "development" {
		log.Printf("connect to http://localhost:%s/ for GraphQL playground", port)
	} else if config.Data.App.Environment == "production" {
		log.Printf("PRODUCTION SERVER RUNNING")
	}

	log.Fatal(http.ListenAndServe(":"+port, router))
}

func listUserTravelPlansHandler(w http.ResponseWriter, r *http.Request) {
	user := middleware.ForContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	if limit <= 0 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}

	plans := make([]dbmodel.TravelPlan, 0)
	if err := database.Connection.Where("user_id = ?", user.ID).Order("updated_at desc").Limit(limit).Find(&plans).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	items := make([]service.TravelPlanSummaryResponse, 0, len(plans))
	for _, plan := range plans {
		items = append(items, service.BuildTravelPlanSummaryResponse(plan))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": items,
	})
}

func getUserTravelPlanHandler(w http.ResponseWriter, r *http.Request) {
	user := middleware.ForContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	idParam := strings.TrimSpace(chi.URLParam(r, "id"))
	planID, err := strconv.Atoi(idParam)
	if err != nil || planID <= 0 {
		http.Error(w, "invalid plan id", http.StatusBadRequest)
		return
	}

	var plan dbmodel.TravelPlan
	plan.ID = uint(planID)
	if err := plan.GetWithDailyPlans(database.Connection); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "plan not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if plan.UserID != user.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	writeJSON(w, http.StatusOK, service.BuildTravelPlanDetailResponse(plan))
}

func writeJSON(w http.ResponseWriter, statusCode int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("failed to write json response: %v", err)
	}
}

// start seeding the database
// seeding is only good for testing, you shouldn't seed in production
// please create one user and relation before seeding
// we should also have pin type and marker type before seeding (might also be also seed-able in the future)
func startSeed() {
	log.Println("start seeding...")
	seed.SeedDatabase()
	log.Println("finished seeding")
}

func fileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit any URL parameters.")
	}

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))
		fs.ServeHTTP(w, r)
	})
}
