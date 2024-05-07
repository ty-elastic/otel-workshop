package main

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"net/http"

	"github.com/gin-gonic/gin"
)

// album represents data about a record album.
type Album struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Artist string  `json:"artist"`
	Price  float64 `json:"price"`
}

const albumsSqlTable = `
	CREATE TABLE ALBUMS(
		ID VARCHAR(10) PRIMARY KEY,
		TITLE text not null,
		ARTIST text not null,
		PRICE float not null
	)
`

// albums slice to seed record album data.
var albums = []Album{
	{ID: "1", Title: "Tubthumper", Artist: "Chumbawumba", Price: 56.99},
	{ID: "2", Title: "Jeru", Artist: "Gerry Mulligan", Price: 17.99},
	{ID: "3", Title: "Sarah Vaughan and Clifford Brown", Artist: "Sarah Vaughan", Price: 39.99},
}

func (c *Catalog) initPostgres() error {
	// build connection string
	url := "postgres://" + os.Getenv("POSTGRES_USER") + ":" + os.Getenv("POSTGRES_PASSWORD") + "@" + os.Getenv("POSTGRES_ADDR") + ":5432/MUSIC"
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return err
	}

	// connect in OTel tracer as middleware
	cfg.ConnConfig.Tracer = otelpgx.NewTracer()
	if cfg.ConnConfig.Tracer == nil {
		return fmt.Errorf("unable to create otelpgx tracer")
	}

	// build config
	conn, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return err
	}
	c.postgres = conn

	// try to create initial table
	_, err = conn.Exec(context.Background(), albumsSqlTable)
	if err != nil {
		// if unable to connect, die and retry
		if _, ok := err.(net.Error); ok {
			log.Fatal("unable to connect to database: ", err)
			os.Exit(1)
		} else {
			log.Warn("unable to create table: ", err)
		}
	}

	// init db
	for _, a := range albums {

		err := c.addAlbum(context.Background(), &a)
		if err != nil {
			log.Warn("unable to insert rows into table: ", err)
		}
	}

	return nil
}

func (c *Catalog) addAlbum(context context.Context, newAlbum *Album) error {
	sqlStatement := `
		INSERT INTO ALBUMS (ID, TITLE, ARTIST, PRICE)
		VALUES ($1, $2, $3, $4)
	`

	// insert new album
	returnval, err := c.postgres.Exec(context, sqlStatement, newAlbum.ID, newAlbum.Title, newAlbum.Artist, newAlbum.Price)
	fmt.Printf("value of returnval %s", returnval)
	if err != nil {
		return err
	}

	return nil
}

// internal function to check auth
func (c *Catalog) checkAuth(ctx *gin.Context) bool {
	// manually create a span
	_, span := c.tracer.Start(ctx.Request.Context(), "checkAuth")
	defer span.End()

	// log with traceid, spanid
	logger.WithContext(ctx.Request.Context()).Info("Checking auth...")

	// increment metric
	c.authCnt.Add(ctx, 1)

	// check if client is requesting intentional errors
	for key, values := range ctx.Request.URL.Query() {
		if key == "error" {
			for _, value := range values {
				if value == "remote401" {
					// set failed span
					span.SetStatus(otelcodes.Error, "unknown user")
					span.RecordError(fmt.Errorf("unknown user"))
					return false
				} else if value == "remoteLatency" {
					// demonstrate span events
					span.AddEvent("Adding latency")
					rnd := rand.Intn(5) + 2
					time.Sleep(time.Duration(rnd) * time.Second)
					// add an attribute to this span so we know why the latency was high
					span.SetAttributes(
						attribute.Int("addedLatency", rnd),
					)
				}
			}
		}
	}

	return true
}

// getAlbums responds with the list of all albums as JSON.
func (c *Catalog) getAlbums(ctx *gin.Context) {
	// get current span context (from auto-instrumentation)
	span := trace.SpanFromContext(ctx.Request.Context())
	span.SetAttributes(
		attribute.Bool("test", true),
	)

	// pull baggage from context (gin otel middleware pulled it from request headers and put it onto context for us)
	traceBaggage := baggage.FromContext(ctx.Request.Context())
	for _, member := range traceBaggage.Members() {
		// set all baggage as span attributes
		span.SetAttributes(
			attribute.String(member.Key(), member.Value()),
		)
	}

	// check if request is authorized
	if authorized := c.checkAuth(ctx); !authorized {
		ctx.IndentedJSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	// query postgres
	rows, err := c.postgres.Query(ctx.Request.Context(), "SELECT * FROM ALBUMS")
	if err != nil {
		logger.WithContext(ctx.Request.Context()).Fatal("unable to query postgres: ", err)
		ctx.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "database down"})
		return
	}
	defer rows.Close()

	// populate in-memory table
	var albums []Album
	// Loop through rows, using Scan to assign column data to struct fields.
	for rows.Next() {
		var album Album
		if err := rows.Scan(&album.ID, &album.Title, &album.Artist, &album.Price); err != nil {
			break
		}
		albums = append(albums, album)
	}
	// check for errors
	if err = rows.Err(); err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "database error"})
		return
	}

	// return albums
	ctx.IndentedJSON(http.StatusOK, albums)
}

// postAlbums adds an album from JSON received in the request body.
func (c *Catalog) postAlbums(ctx *gin.Context) {
	var newAlbum Album

	// bind POST body to album object
	if err := ctx.BindJSON(&newAlbum); err != nil {
		return
	}

	// insert new album
	err := c.addAlbum(ctx.Request.Context(), &newAlbum)
	if err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, gin.H{"message": "unable to insert rows"})
		return
	}

	ctx.IndentedJSON(http.StatusCreated, newAlbum)
}

// getAlbumByID locates the album whose ID value matches the id
func (c *Catalog) getAlbumByID(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		ctx.IndentedJSON(http.StatusBadRequest, gin.H{"message": "id not specified"})
		return
	}

	row := c.postgres.QueryRow(ctx.Request.Context(), "SELECT * FROM ALBUMS WHERE ID = $1", id)
	var album Album
	if err := row.Scan(&album.ID, &album.Title, &album.Artist, &album.Price); err != nil {
		logger.WithContext(ctx.Request.Context()).Warn("unable to find album: ", err)
		ctx.IndentedJSON(http.StatusNotFound, gin.H{"message": "row not found"})
		return
	}

	ctx.IndentedJSON(http.StatusOK, album)
}
