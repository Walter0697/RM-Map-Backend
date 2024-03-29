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
	current_version := "2.8.0"
	// notes := []string{
	// 	"[b]New Feature:",
	// 	"Adding RoroadList, alongside with previous roroadlists revoke!",
	// 	"Sorting Markers will be available in Filter Page",
	// 	"[b]Quality Of Life",
	// 	"Clicking Address will now open Google Map",
	// }
	// notes := []string{
	// 	"[b]Bug Fixed:",
	// 	"Openrice scrapper removing and editing issue",
	// }
	notes := []string{
		"[b]New Feature:",
		"Group markers by country and location",
		"Watched Movies List",
		"LatLon Form for adding location in map",
		"[b]Layout:",
		"Most listing buttons are now at Setting page instead of Home page",
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

	// for non dynamic asset that is required when nothing is set
	assetsDir := http.Dir(filepath.Join(workDir, "assets"))
	fileServer(router, "/image/static", assetsDir)

	if config.Data.App.Environment == "development" {
		router.Handle("/", playground.Handler("GraphQL playground", "/query"))
	}

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
