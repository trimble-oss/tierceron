package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	eUtils "github.com/trimble-oss/tierceron/utils"
	"github.com/trimble-oss/tierceron/utils/mlock"
	twp "github.com/trimble-oss/tierceron/webapi/rpc/apinator"
	"github.com/trimble-oss/tierceron/webapi/server"

	jwt "github.com/golang-jwt/jwt"
	rtr "github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
)

// Must be the same secret used to sign valid tokens
var s *server.Server
var noAuthRoutes = map[string]bool{
	"APILogin":        true,
	"CheckConnection": true,
	"GetStatus":       true,
	"GetVaultTokens":  true,
	"InitVault":       true,
	"ResetServer":     true,
	"Unseal":          true,
	"UpdateAPI":       true,
	"Environments":    true,
}

// Handle auth tokens through POST request and route without auth through GET request
func authrouter(restHandler http.Handler, isAuth bool) *rtr.Router {
	router := rtr.New()
	// Simply route
	noauth := func(w http.ResponseWriter, r *http.Request, ps rtr.Params) {
		s.Log.SetPrefix("[INFO]")
		s.Log.Printf("Incoming request %s %s\n \tfrom %s\n", r.Method, r.URL.String(), r.RemoteAddr)
		s.Log.Println("Handling with no auth")
		restHandler.ServeHTTP(w, r)
	}
	auth := func(w http.ResponseWriter, r *http.Request, ps rtr.Params) {
		// Switch to noauth if this is a login request
		if noAuthRoutes[ps.ByName("method")] {
			noauth(w, r, ps)
			return
		}

		var errMsg string

		s.Log.SetPrefix("[INFO]")
		s.Log.Printf("Incoming request %s %s\n \tFrom %s\n", r.Method, r.URL.String(), r.RemoteAddr)
		s.Log.SetPrefix("[ERROR]")
		authString := r.Header.Get("Authorization")

		if len(authString) > 0 { // Ensure a token was actually sent
			splitAuth := strings.SplitN(authString, " ", 2)
			if splitAuth[0] == "Bearer" {
				token, err := jwt.Parse(splitAuth[1], func(token *jwt.Token) (interface{}, error) { // Parse token and verify formatting
					if _, ok := token.Method.(*jwt.SigningMethodHMAC); ok {
						return s.TrcAPITokenSecret, nil
					}
					parseErr := fmt.Errorf("Unexpected singing method %v", token.Header["alg"])
					s.Log.Println(parseErr)
					return nil, parseErr
				})
				if err == nil { // Continue if token parsed without error
					if claims, ok := token.Claims.(jwt.MapClaims); ok { // Verify token claim formatting
						err = claims.Valid()
						if err == nil { // Verify that token had valid issuing time/date
							if claims["iss"] != "Viewpoint, Inc." { // Verify issuer
								errMsg = "Invalid token issuer: " + claims["iss"].(string)
								http.Error(w, errMsg, 401)
								s.Log.Println(errMsg)
								return
							} else if claims["aud"] != "Viewpoint Vault WebAPI" { // Verify audience
								errMsg = "Token issued for different audience: " + claims["aud"].(string)
								http.Error(w, errMsg, 401)
								s.Log.Println(errMsg)
								return
							}
							// Output token info and pass request to twirp server
							s.Log.SetPrefix("[INFO]")
							s.Log.Printf("Request authorized for %v with ID %v\n", claims["name"], claims["sub"])
							ctx := r.Context()
							restHandler.ServeHTTP(w, r.WithContext(context.WithValue(ctx, "user", claims["sub"])))
							return
						}
						// Before issue time, after expiration, or before validity time
						http.Error(w, err.Error(), 401)
						s.Log.Printf("%d: %s", 401, err.Error())
						return
					}
					// Token claims not in json format
					errMsg = "Format error with auth token claims"
					http.Error(w, errMsg, 401)
					s.Log.Printf("%d: %s", 401, errMsg)
					return
				}
				// Error when parsing token. Pass back a generalized error for formatting
				errMsg = "Invalid token: " + err.Error()
				http.Error(w, errMsg, 401)
				s.Log.Printf("%d: %s\n", 401, errMsg)
				return
			}
			// Auth method passed but is not a bearer token
			errMsg = "Invalid auth method " + splitAuth[0]
			http.Error(w, errMsg, 401)
			s.Log.Printf("%d: %s", 401, errMsg)
			return
		}
		// No token to authenticate against
		errMsg = "Missing auth token"
		http.Error(w, errMsg, 401)
		s.Log.Printf("%d: %s", 401, errMsg)
		return

	}
	gql := func(w http.ResponseWriter, r *http.Request, ps rtr.Params) {
		query := strings.Replace(r.URL.Query().Get("query"), `"`, `\"`, -1)
		body := `{"query": "` + query + `"}`
		GQLReq, err := http.NewRequest("POST",
			coreopts.GetVaultHost()+"/twirp/viewpoint.whoville.apinator.EnterpriseServiceBroker/GraphQL",
			strings.NewReader(body))
		GQLReq.Header = r.Header
		if err != nil {
			s.Log.Println(err)
			return
		}
		GQLReq.Header["Content-Type"] = []string{"application/json"}
		ctx := GQLReq.Context()
		ctx = context.WithValue(ctx, "Authorization", GQLReq.Header["Authorization"])
		GQLReq = GQLReq.WithContext(ctx)
		if isAuth {
			auth(w, GQLReq, ps)
		} else {
			noauth(w, GQLReq, ps)
		}
	}
	if isAuth {
		router.GET("/twirp/:service/:method", noauth)
		router.POST("/twirp/:service/:method", auth)
	} else {
		router.GET("/twirp/:service/:method", noauth)
		router.POST("/twirp/:service/:method", noauth)
	}
	router.GET("/graphql", gql)
	router.GET("/auth", auth)

	uiEndpoint := func(w http.ResponseWriter, r *http.Request, ps rtr.Params) {
		http.ServeFile(w, r, "public/index.html")
	}
	gqlEndpoint := func(w http.ResponseWriter, r *http.Request, ps rtr.Params) {
		s.InitGQL()
		http.ServeFile(w, r, "public/index.html")
	}
	router.GET("/", uiEndpoint)
	router.GET("/login", uiEndpoint)
	router.GET("/values", gqlEndpoint)
	router.GET("/sealed", uiEndpoint)
	router.GET("/sessions", gqlEndpoint)
	router.ServeFiles("/public/*filepath", http.Dir("public"))
	return router
}

// declare global variale for local hosting
var localHost bool

// environments
var environments = []string{"dev", "QA", "RQA", "auto", "performance", "servicepack", "itdev"}
var prodEnvironments = []string{"staging", "prod"}
var webAPIProdEnvironments = []string{"staging"}

func main() {
	fmt.Println("Version: " + "1.1")
	addrPtr := flag.String("addr", coreopts.GetVaultHostPort(), "API endpoint for the vault")
	tokenPtr := flag.String("token", "", "Vault access token")
	logPathPtr := flag.String("log", "/etc/opt/"+coreopts.GetFolderPrefix()+"API/server.log", "Log file path for this server")
	tlsPathPtr := flag.String("tlsPath", "../vault/certs", "Path to server certificate and private key")
	authPtr := flag.Bool("auth", true, "Run with auth enabled?")
	localPtr := flag.Bool("local", false, "Run locally")
	prodPtr := flag.Bool("production", false, "Run in production mode")

	flag.Parse()

	s = server.NewServer(*addrPtr, *tokenPtr)
	localHost = *localPtr
	config := &eUtils.DriverConfig{ExitOnFailure: true}

	f, err := os.OpenFile(*logPathPtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	eUtils.CheckError(config, err, true)
	s.Log.SetOutput(f)
	mlock.Mlock(s.Log)

	status, err := s.GetStatus(context.Background(), nil)
	eUtils.LogErrorObject(config, err, true)

	if !status.Sealed && s.VaultToken != "" {
		s.Log.Println("Vault is unsealed. Initializing GQL")
		s.InitGQL()
	}
	if *prodPtr {
		server.SelectedEnvironment = prodEnvironments
		server.SelectedWebEnvironment = webAPIProdEnvironments
	} else {
		server.SelectedEnvironment = environments
		server.SelectedWebEnvironment = environments
	}

	twirpHandler := twp.NewEnterpriseServiceBrokerServer(s, nil)
	//twirpHandler.
	router := authrouter(twirpHandler, *authPtr)
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "DELETE", "PUT", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	})

	// specify ports
	port := ":80"
	securePort := ":443"
	if localHost {
		port = ":9012"
		securePort = ":9014"
	}

	// http request redirects
	go func() {
		s.Log.Fatal(http.ListenAndServe(port, http.HandlerFunc(redirectToTLS)))
	}()
	fmt.Println("Server initialized and running")
	s.Log.Printf("Listening for HTTPS requests on port %s\n", securePort)
	s.Log.Printf("Redirecting HTTP requests from port %s to HTTPS on port %s\n", port, securePort)
	s.Log.Fatal(http.ListenAndServeTLS(securePort, *tlsPathPtr+"/serv_cert.pem", *tlsPathPtr+"/serv_key.pem", c.Handler(router)))
}

func redirectToTLS(w http.ResponseWriter, r *http.Request) {
	redirectURL := coreopts.GetVaultHost() + r.URL.Path
	if localHost {
		redirectURL = coreopts.GetLocalHost() + r.URL.Path
	}
	if len(r.URL.RawQuery) > 0 {
		redirectURL += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
}
