package main

import (
	"log"
	"mapmarker/backend/config"
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
	"strings"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi"
	"github.com/go-chi/cors"
)

func main() {
	config.Init()
	config.SetupGoGuardian()
	database.Init()
	dbmodel.AutoMigration()
	service.StartAPIKeyCleanupWorker()

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

	prepareReleaseNote()

	startServer()
}

// if there is release note to add, add it
// if there isn't, don't
func prepareReleaseNote() {
	current_version := "2.9.4"
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
		r.Post("/markers", service.IntegrationCreateMarkerHandler)
		r.Put("/markers/{id}", service.IntegrationUpdateMarkerHandler)
		r.Get("/schedules", service.IntegrationListSchedulesHandler)
		r.Post("/schedules", service.IntegrationCreateScheduleHandler)
		r.Get("/stations", service.IntegrationListStationsHandler)
		r.Put("/stations", service.IntegrationUpdateStationHandler)
		r.Get("/settings/pins", service.IntegrationListSettingsPinsHandler)
		r.Get("/settings/marker-types", service.IntegrationListSettingsMarkerTypesHandler)
		r.Get("/settings/default-pins", service.IntegrationListSettingsDefaultPinsHandler)
		r.Put("/settings/default-pins/{label}", service.IntegrationUpdateSettingsDefaultPinHandler)
	})
	router.Route("/admin", func(r chi.Router) {
		r.Get("/cleanup/markers", service.AdminCleanupListMarkersHandler)
		r.Get("/cleanup/schedules", service.AdminCleanupListSchedulesHandler)
		r.Delete("/cleanup/markers/{id}", service.AdminCleanupDeleteMarkerHandler)
		r.Delete("/cleanup/schedules/{id}", service.AdminCleanupDeleteScheduleHandler)
		r.Post("/cleanup/jobs", service.AdminCleanupScheduleJobHandler)
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
