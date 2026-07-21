package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ctrl-research/waypoint/internal/auth"
	"github.com/ctrl-research/waypoint/internal/geocode"
	"github.com/ctrl-research/waypoint/internal/store"
)

// MCP support (#92): a Streamable-HTTP MCP server at /mcp so LLM clients
// can backfill and manage trips. Every request authenticates with the
// user's bearer token; tools run with exactly that user's access.

// ---- token management (mirrors the calendar feed token) ----------------------

func (api *tripsAPI) getMCPToken(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"token": user.McpToken})
}

func (api *tripsAPI) createMCPToken(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	buf := make([]byte, 32)
	rand.Read(buf)
	token := base64.RawURLEncoding.EncodeToString(buf)
	if _, err := api.users.SetMCPToken(r.Context(), user.ID, &token); err != nil {
		apiInternalError(w, "set mcp token", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": token})
}

func (api *tripsAPI) deleteMCPToken(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFrom(r.Context())
	if _, err := api.users.SetMCPToken(r.Context(), user.ID, nil); err != nil {
		apiInternalError(w, "clear mcp token", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- the MCP endpoint ---------------------------------------------------------

// mcpHandler authenticates the bearer token and serves the MCP session
// with that user attached to the request context.
func (api *tripsAPI) mcpHandler(geo *geocode.Client) http.Handler {
	srv := mcp.NewServer(&mcp.Implementation{Name: "waypoint", Version: "1.0.0"}, nil)
	api.registerMCPTools(srv, geo)
	streamable := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" || token == r.Header.Get("Authorization") {
			apiError(w, http.StatusUnauthorized, "unauthenticated", "missing bearer token")
			return
		}
		user, err := api.users.ByMCPToken(r.Context(), token)
		if errors.Is(err, store.ErrNotFound) {
			apiError(w, http.StatusUnauthorized, "unauthenticated", "unknown token")
			return
		}
		if err != nil {
			apiInternalError(w, "lookup mcp token", err)
			return
		}
		streamable.ServeHTTP(w, r.WithContext(auth.WithUserContext(r.Context(), user)))
	})
}

// mcpTrip resolves a trip id and enforces the caller's minimum role.
func (api *tripsAPI) mcpTrip(ctx context.Context, tripID, min string) (store.Trip, error) {
	user, ok := auth.UserFrom(ctx)
	if !ok {
		return store.Trip{}, errors.New("unauthenticated")
	}
	id, err := uuid.Parse(tripID)
	if err != nil {
		return store.Trip{}, errors.New("tripId is not a valid id")
	}
	trip, role, err := api.trips.WithRole(ctx, id, user.ID)
	if errors.Is(err, store.ErrNotFound) || (err == nil && role == "") {
		return store.Trip{}, errors.New("trip not found")
	}
	if err != nil {
		return store.Trip{}, err
	}
	if roleRank(role) < roleRank(min) {
		return store.Trip{}, errors.New("your role on this trip does not allow that")
	}
	return trip, nil
}

func parseMCPDate(s, field string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	d, err := time.Parse(dateFormat, s)
	if err != nil {
		return nil, fmt.Errorf("%s must be YYYY-MM-DD", field)
	}
	return &d, nil
}

// ---- tool shapes ----------------------------------------------------------------

type mcpTripSummary struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Status      string  `json:"status" jsonschema:"planning, active, or completed"`
	StartDate   *string `json:"startDate"`
	EndDate     *string `json:"endDate"`
}

func toMCPTrip(t store.Trip) mcpTripSummary {
	return mcpTripSummary{
		ID: t.ID.String(), Title: t.Title, Description: t.Description,
		Status: string(t.Status), StartDate: formatDate(t.StartDate), EndDate: formatDate(t.EndDate),
	}
}

type mcpArea struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Kind          string   `json:"kind,omitempty" jsonschema:"scale of the place: country, city, town…"`
	Lat           *float64 `json:"lat,omitempty"`
	Lon           *float64 `json:"lon,omitempty"`
	ArrivalDate   *string  `json:"arrivalDate,omitempty"`
	DepartureDate *string  `json:"departureDate,omitempty"`
}

func toMCPArea(s store.Stop) mcpArea {
	return mcpArea{
		ID: s.ID.String(), Name: s.Name, Kind: s.Kind, Lat: s.Lat, Lon: s.Lon,
		ArrivalDate: formatDate(s.ArrivalDate), DepartureDate: formatDate(s.DepartureDate),
	}
}

type mcpItem struct {
	ID                 string `json:"id"`
	Title              string `json:"title"`
	Day                string `json:"day"`
	Category           string `json:"category"`
	StartTime          string `json:"startTime,omitempty"`
	EndTime            string `json:"endTime,omitempty"`
	Timezone           string `json:"timezone,omitempty" jsonschema:"IANA timezone name, e.g. America/Vancouver; omit to use WAYPOINT_TIMEZONE or floating time"`
	AreaID             string `json:"areaId,omitempty"`
	DestinationAreaID  string `json:"destinationAreaId,omitempty"`
	Address            string `json:"address,omitempty"`
	DestinationAddress string `json:"destinationAddress,omitempty"`
	Notes              string `json:"notes,omitempty"`
	Layer              string `json:"layer,omitempty"`
}

func toMCPItem(it store.ItineraryItem, layerNames map[uuid.UUID]string) mcpItem {
	out := mcpItem{
		ID: it.ID.String(), Title: it.Title, Day: it.Day.Format(dateFormat),
		Category: string(it.Category), StartTime: it.StartTime, EndTime: it.EndTime,
		Address: it.Address, DestinationAddress: it.DestinationAddress, Notes: it.Notes,
		Layer: layerNames[it.LayerID],
	}
	if it.Timezone != nil {
		out.Timezone = *it.Timezone
	}
	if it.StopID != nil {
		out.AreaID = it.StopID.String()
	}
	if it.DestinationStopID != nil {
		out.DestinationAreaID = it.DestinationStopID.String()
	}
	return out
}

// ---- tools ----------------------------------------------------------------------

func (api *tripsAPI) registerMCPTools(srv *mcp.Server, geo *geocode.Client) {
	type empty struct{}

	type listTripsOut struct {
		Trips []mcpTripSummary `json:"trips"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_trips",
		Description: "List every trip the user can see, with ids for use in other tools.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ empty) (*mcp.CallToolResult, listTripsOut, error) {
		user, _ := auth.UserFrom(ctx)
		trips, err := api.trips.ListAccessible(ctx, user.ID)
		if err != nil {
			return nil, listTripsOut{}, err
		}
		out := listTripsOut{Trips: make([]mcpTripSummary, 0, len(trips))}
		for _, t := range trips {
			out.Trips = append(out.Trips, toMCPTrip(t.Trip))
		}
		return nil, out, nil
	})

	type getTripIn struct {
		TripID string `json:"tripId"`
	}
	type getTripOut struct {
		Trip   mcpTripSummary `json:"trip"`
		Areas  []mcpArea      `json:"areas"`
		Items  []mcpItem      `json:"items"`
		Layers []string       `json:"layers"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_trip",
		Description: "Full detail of one trip: its areas (countries/cities visited, in route order), itinerary items, and layer names.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getTripIn) (*mcp.CallToolResult, getTripOut, error) {
		trip, err := api.mcpTrip(ctx, in.TripID, "viewer")
		if err != nil {
			return nil, getTripOut{}, err
		}
		stops, err := api.trips.ListStops(ctx, trip.ID)
		if err != nil {
			return nil, getTripOut{}, err
		}
		items, err := api.trips.ListItems(ctx, trip.ID)
		if err != nil {
			return nil, getTripOut{}, err
		}
		layers, err := api.trips.ListLayers(ctx, trip.ID)
		if err != nil {
			return nil, getTripOut{}, err
		}
		out := getTripOut{Trip: toMCPTrip(trip), Areas: []mcpArea{}, Items: []mcpItem{}, Layers: []string{}}
		for _, s := range stops {
			out.Areas = append(out.Areas, toMCPArea(s))
		}
		names := map[uuid.UUID]string{}
		for _, l := range layers {
			names[l.ID] = l.Name
			out.Layers = append(out.Layers, l.Name)
		}
		for _, it := range items {
			out.Items = append(out.Items, toMCPItem(it, names))
		}
		return nil, out, nil
	})

	type createTripIn struct {
		Title       string `json:"title"`
		Description string `json:"description,omitempty"`
		StartDate   string `json:"startDate,omitempty" jsonschema:"YYYY-MM-DD"`
		EndDate     string `json:"endDate,omitempty" jsonschema:"YYYY-MM-DD"`
		Status      string `json:"status,omitempty" jsonschema:"planning (default), active, or completed — use completed when backfilling past trips"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_trip",
		Description: "Create a trip. Returns the trip with its id for adding areas and items.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createTripIn) (*mcp.CallToolResult, mcpTripSummary, error) {
		user, _ := auth.UserFrom(ctx)
		if strings.TrimSpace(in.Title) == "" {
			return nil, mcpTripSummary{}, errors.New("title is required")
		}
		params := store.TripParams{Title: in.Title, Description: in.Description, Status: store.TripPlanning}
		if in.Status != "" {
			if !store.ValidTripStatus(in.Status) {
				return nil, mcpTripSummary{}, errors.New("status must be planning, active, or completed")
			}
			params.Status = store.TripStatus(in.Status)
		}
		var err error
		if params.StartDate, err = parseMCPDate(in.StartDate, "startDate"); err != nil {
			return nil, mcpTripSummary{}, err
		}
		if params.EndDate, err = parseMCPDate(in.EndDate, "endDate"); err != nil {
			return nil, mcpTripSummary{}, err
		}
		if params.StartDate != nil && params.EndDate != nil && params.EndDate.Before(*params.StartDate) {
			return nil, mcpTripSummary{}, errors.New("endDate must not be before startDate")
		}
		trip, err := api.trips.Create(ctx, user.ID, params)
		if err != nil {
			return nil, mcpTripSummary{}, err
		}
		return nil, toMCPTrip(trip), nil
	})

	type addAreaIn struct {
		TripID        string   `json:"tripId"`
		Name          string   `json:"name" jsonschema:"the country, city, or region name"`
		Lat           *float64 `json:"lat,omitempty" jsonschema:"omit to geocode the name automatically"`
		Lon           *float64 `json:"lon,omitempty"`
		ArrivalDate   string   `json:"arrivalDate,omitempty" jsonschema:"YYYY-MM-DD"`
		DepartureDate string   `json:"departureDate,omitempty" jsonschema:"YYYY-MM-DD"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "add_area",
		Description: "Add an area (country/city/region) to a trip's route, in visit order. Without lat/lon the name is geocoded automatically.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in addAreaIn) (*mcp.CallToolResult, mcpArea, error) {
		trip, err := api.mcpTrip(ctx, in.TripID, "editor")
		if err != nil {
			return nil, mcpArea{}, err
		}
		if strings.TrimSpace(in.Name) == "" {
			return nil, mcpArea{}, errors.New("name is required")
		}
		params := store.StopParams{Name: in.Name, Lat: in.Lat, Lon: in.Lon}
		if params.ArrivalDate, err = parseMCPDate(in.ArrivalDate, "arrivalDate"); err != nil {
			return nil, mcpArea{}, err
		}
		if params.DepartureDate, err = parseMCPDate(in.DepartureDate, "departureDate"); err != nil {
			return nil, mcpArea{}, err
		}
		if params.Lat == nil {
			if results, err := geo.Search(ctx, in.Name, 1, ""); err == nil && len(results) > 0 {
				params.Lat, params.Lon = &results[0].Lat, &results[0].Lon
				params.Kind = results[0].Type
			}
		}
		stop, err := api.trips.CreateStop(ctx, trip.ID, params)
		if err != nil {
			return nil, mcpArea{}, err
		}
		return nil, toMCPArea(stop), nil
	})

	type deleteAreaIn struct {
		TripID string `json:"tripId"`
		AreaID string `json:"areaId"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_area",
		Description: "Remove an area from a trip. Itinerary items that referenced it stay, unlinked.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in deleteAreaIn) (*mcp.CallToolResult, empty, error) {
		trip, err := api.mcpTrip(ctx, in.TripID, "editor")
		if err != nil {
			return nil, empty{}, err
		}
		id, err := uuid.Parse(in.AreaID)
		if err != nil {
			return nil, empty{}, errors.New("areaId is not a valid id")
		}
		if err := api.trips.DeleteStop(ctx, trip.ID, id); err != nil {
			return nil, empty{}, err
		}
		return nil, empty{}, nil
	})

	type addItemIn struct {
		TripID             string `json:"tripId"`
		Title              string `json:"title"`
		Day                string `json:"day" jsonschema:"YYYY-MM-DD"`
		Category           string `json:"category,omitempty" jsonschema:"activity (default), food, lodging, transport, flight, train, or other"`
		StartTime          string `json:"startTime,omitempty" jsonschema:"HH:MM, 24h"`
		EndTime            string `json:"endTime,omitempty" jsonschema:"HH:MM; for flights/trains this is the arrival time"`
		Timezone           string `json:"timezone,omitempty" jsonschema:"IANA timezone name, e.g. America/Vancouver; omit to use WAYPOINT_TIMEZONE or floating time"`
		AreaID             string `json:"areaId,omitempty" jsonschema:"the area this happens in; for flights/trains the departure area"`
		DestinationAreaID  string `json:"destinationAreaId,omitempty" jsonschema:"flights/trains only: the arrival area"`
		Address            string `json:"address,omitempty" jsonschema:"venue address; for flights/trains the departure station/airport"`
		DestinationAddress string `json:"destinationAddress,omitempty" jsonschema:"flights/trains only: arrival station/airport"`
		Notes              string `json:"notes,omitempty"`
		Layer              string `json:"layer,omitempty" jsonschema:"layer name; omit for the trip's Main layer, unknown names are created"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "add_item",
		Description: "Add an itinerary item to a trip day. Flights/trains are legs: give them departure (areaId/address) and arrival (destinationAreaId/destinationAddress) plus start/end times.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in addItemIn) (*mcp.CallToolResult, mcpItem, error) {
		trip, err := api.mcpTrip(ctx, in.TripID, "editor")
		if err != nil {
			return nil, mcpItem{}, err
		}
		req := itemRequest{}
		if in.Title != "" {
			req.Title = &in.Title
		}
		if in.Day != "" {
			req.Day = &in.Day
		}
		if in.Category != "" {
			req.Category = &in.Category
		}
		if in.StartTime != "" {
			req.StartTime = &in.StartTime
		}
		if in.EndTime != "" {
			req.EndTime = &in.EndTime
		}
		if in.Timezone != "" {
			req.Timezone = &in.Timezone
		}
		if in.Address != "" {
			req.Address = &in.Address
		}
		if in.DestinationAddress != "" {
			req.DestinationAddress = &in.DestinationAddress
		}
		if in.Notes != "" {
			req.Notes = &in.Notes
		}
		if in.AreaID != "" {
			id, err := uuid.Parse(in.AreaID)
			if err != nil {
				return nil, mcpItem{}, errors.New("areaId is not a valid id")
			}
			req.StopID = &id
		}
		if in.DestinationAreaID != "" {
			id, err := uuid.Parse(in.DestinationAreaID)
			if err != nil {
				return nil, mcpItem{}, errors.New("destinationAreaId is not a valid id")
			}
			req.DestinationStopID = &id
		}
		var params store.ItineraryItemParams
		if err := req.merge(&params); err != nil {
			return nil, mcpItem{}, err
		}
		if ok, err := api.stopBelongsToTrip2(ctx, trip.ID, params.StopID); err != nil || !ok {
			if err != nil {
				return nil, mcpItem{}, err
			}
			return nil, mcpItem{}, errors.New("areaId does not belong to this trip")
		}
		if ok, err := api.stopBelongsToTrip2(ctx, trip.ID, params.DestinationStopID); err != nil || !ok {
			if err != nil {
				return nil, mcpItem{}, err
			}
			return nil, mcpItem{}, errors.New("destinationAreaId does not belong to this trip")
		}
		layer, err := api.mcpResolveLayer(ctx, trip.ID, in.Layer)
		if err != nil {
			return nil, mcpItem{}, err
		}
		params.LayerID = layer.ID
		item, err := api.trips.CreateItem(ctx, trip.ID, params)
		if err != nil {
			return nil, mcpItem{}, err
		}
		return nil, toMCPItem(item, map[uuid.UUID]string{layer.ID: layer.Name}), nil
	})

	type deleteItemIn struct {
		TripID string `json:"tripId"`
		ItemID string `json:"itemId"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_item",
		Description: "Delete one itinerary item.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in deleteItemIn) (*mcp.CallToolResult, empty, error) {
		trip, err := api.mcpTrip(ctx, in.TripID, "editor")
		if err != nil {
			return nil, empty{}, err
		}
		id, err := uuid.Parse(in.ItemID)
		if err != nil {
			return nil, empty{}, errors.New("itemId is not a valid id")
		}
		if err := api.trips.DeleteItem(ctx, trip.ID, id); err != nil {
			return nil, empty{}, err
		}
		return nil, empty{}, nil
	})

	// ---- update_trip --------------------------------------------------------

	type updateTripIn struct {
		TripID      string `json:"tripId"`
		Title       string `json:"title,omitempty"`
		Description string `json:"description,omitempty"`
		StartDate   string `json:"startDate,omitempty" jsonschema:"YYYY-MM-DD"`
		EndDate     string `json:"endDate,omitempty" jsonschema:"YYYY-MM-DD"`
		Status      string `json:"status,omitempty" jsonschema:"planning, active, or completed"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_trip",
		Description: "Update a trip's title, description, dates, or status. Only provided fields are changed.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateTripIn) (*mcp.CallToolResult, mcpTripSummary, error) {
		trip, err := api.mcpTrip(ctx, in.TripID, "editor")
		if err != nil {
			return nil, mcpTripSummary{}, err
		}
		params := store.TripParams{
			Title: trip.Title, Description: trip.Description, Status: trip.Status,
			StartDate: trip.StartDate, EndDate: trip.EndDate, CoverPhoto: trip.CoverPhoto,
		}
		if in.Title != "" {
			params.Title = in.Title
		}
		if in.Description != "" {
			params.Description = in.Description
		}
		if in.Status != "" {
			if !store.ValidTripStatus(in.Status) {
				return nil, mcpTripSummary{}, errors.New("status must be planning, active, or completed")
			}
			params.Status = store.TripStatus(in.Status)
		}
		var parsedStart, parsedEnd *time.Time
		if in.StartDate != "" {
			parsedStart, err = parseMCPDate(in.StartDate, "startDate")
			if err != nil {
				return nil, mcpTripSummary{}, err
			}
			params.StartDate = parsedStart
		}
		if in.EndDate != "" {
			parsedEnd, err = parseMCPDate(in.EndDate, "endDate")
			if err != nil {
				return nil, mcpTripSummary{}, err
			}
			params.EndDate = parsedEnd
		}
		if params.StartDate != nil && params.EndDate != nil && params.EndDate.Before(*params.StartDate) {
			return nil, mcpTripSummary{}, errors.New("endDate must not be before startDate")
		}
		updated, err := api.trips.Update(ctx, trip.ID, params)
		if err != nil {
			return nil, mcpTripSummary{}, err
		}
		return nil, toMCPTrip(updated), nil
	})

	// ---- update_area --------------------------------------------------------

	type updateAreaIn struct {
		TripID        string   `json:"tripId"`
		AreaID        string   `json:"areaId"`
		Name          string   `json:"name,omitempty"`
		Lat           *float64 `json:"lat,omitempty"`
		Lon           *float64 `json:"lon,omitempty"`
		ArrivalDate   string   `json:"arrivalDate,omitempty" jsonschema:"YYYY-MM-DD"`
		DepartureDate string   `json:"departureDate,omitempty" jsonschema:"YYYY-MM-DD"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_area",
		Description: "Update an area's name, coordinates, or dates. Only provided fields are changed.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateAreaIn) (*mcp.CallToolResult, mcpArea, error) {
		trip, err := api.mcpTrip(ctx, in.TripID, "editor")
		if err != nil {
			return nil, mcpArea{}, err
		}
		stopID, err := uuid.Parse(in.AreaID)
		if err != nil {
			return nil, mcpArea{}, errors.New("areaId is not a valid id")
		}
		stop, err := api.trips.StopByID(ctx, trip.ID, stopID)
		if err != nil {
			return nil, mcpArea{}, err
		}
		params := store.StopParams{
			Name: stop.Name, Lat: stop.Lat, Lon: stop.Lon,
			ArrivalDate: stop.ArrivalDate, DepartureDate: stop.DepartureDate,
			Notes: stop.Notes, Kind: stop.Kind,
		}
		if in.Name != "" {
			params.Name = in.Name
		}
		if in.Lat != nil {
			params.Lat = in.Lat
		}
		if in.Lon != nil {
			params.Lon = in.Lon
		}
		if in.ArrivalDate != "" {
			params.ArrivalDate, err = parseMCPDate(in.ArrivalDate, "arrivalDate")
			if err != nil {
				return nil, mcpArea{}, err
			}
		}
		if in.DepartureDate != "" {
			params.DepartureDate, err = parseMCPDate(in.DepartureDate, "departureDate")
			if err != nil {
				return nil, mcpArea{}, err
			}
		}
		updated, err := api.trips.UpdateStop(ctx, trip.ID, stopID, params)
		if err != nil {
			return nil, mcpArea{}, err
		}
		return nil, toMCPArea(updated), nil
	})

	// ---- update_item --------------------------------------------------------

	type updateItemIn struct {
		TripID             string `json:"tripId"`
		ItemID             string `json:"itemId"`
		Title              string `json:"title,omitempty"`
		Day                string `json:"day,omitempty" jsonschema:"YYYY-MM-DD"`
		Category           string `json:"category,omitempty" jsonschema:"activity, food, lodging, transport, flight, train, or other"`
		StartTime          string `json:"startTime,omitempty" jsonschema:"HH:MM, 24h"`
		EndTime            string `json:"endTime,omitempty" jsonschema:"HH:MM"`
		AreaID             string `json:"areaId,omitempty" jsonschema:"the area this happens in"`
		DestinationAreaID  string `json:"destinationAreaId,omitempty" jsonschema:"flights/trains only: the arrival area"`
		Address            string `json:"address,omitempty"`
		DestinationAddress string `json:"destinationAddress,omitempty"`
		Notes              string `json:"notes,omitempty"`
		Layer              string `json:"layer,omitempty" jsonschema:"layer name; omit to keep current layer"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_item",
		Description: "Update an itinerary item. Only provided fields are changed; layer name can be moved.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateItemIn) (*mcp.CallToolResult, mcpItem, error) {
		trip, err := api.mcpTrip(ctx, in.TripID, "editor")
		if err != nil {
			return nil, mcpItem{}, err
		}
		itemID, err := uuid.Parse(in.ItemID)
		if err != nil {
			return nil, mcpItem{}, errors.New("itemId is not a valid id")
		}
		current, err := api.trips.ItemByID(ctx, trip.ID, itemID)
		if err != nil {
			return nil, mcpItem{}, err
		}
		params := store.ItineraryItemParams{
			StopID:             current.StopID,
			DestinationStopID:  current.DestinationStopID,
			OriginHomeID:       current.OriginHomeID,
			DestinationHomeID:  current.DestinationHomeID,
			Day:                current.Day,
			StartTime:          current.StartTime,
			EndTime:            current.EndTime,
			Title:              current.Title,
			Category:           current.Category,
			Notes:              current.Notes,
			CostCents:          current.CostCents,
			Currency:           current.Currency,
			Address:            current.Address,
			Lat:                current.Lat,
			Lon:                current.Lon,
			LayerID:            current.LayerID,
			DestinationAddress: current.DestinationAddress,
			DestinationLat:     current.DestinationLat,
			DestinationLon:     current.DestinationLon,
		}
		if in.Title != "" {
			params.Title = in.Title
		}
		if in.Day != "" {
			day, err := parseMCPDate(in.Day, "day")
			if err != nil {
				return nil, mcpItem{}, err
			}
			params.Day = *day
		}
		if in.Category != "" {
			if !store.ValidItineraryCategory(in.Category) {
				return nil, mcpItem{}, errors.New("category must be activity, food, lodging, transport, flight, train, or other")
			}
			params.Category = store.ItineraryCategory(in.Category)
		}
		if in.StartTime != "" {
			params.StartTime = in.StartTime
		}
		if in.EndTime != "" {
			params.EndTime = in.EndTime
		}
		if in.Address != "" {
			params.Address = in.Address
		}
		if in.DestinationAddress != "" {
			params.DestinationAddress = in.DestinationAddress
		}
		if in.Notes != "" {
			params.Notes = in.Notes
		}
		if in.AreaID != "" {
			id, err := uuid.Parse(in.AreaID)
			if err != nil {
				return nil, mcpItem{}, errors.New("areaId is not a valid id")
			}
			params.StopID = &id
		}
		if in.DestinationAreaID != "" {
			id, err := uuid.Parse(in.DestinationAreaID)
			if err != nil {
				return nil, mcpItem{}, errors.New("destinationAreaId is not a valid id")
			}
			params.DestinationStopID = &id
		}
		if ok, err := api.stopBelongsToTrip2(ctx, trip.ID, params.StopID); err != nil || !ok {
			if err != nil {
				return nil, mcpItem{}, err
			}
			return nil, mcpItem{}, errors.New("areaId does not belong to this trip")
		}
		if ok, err := api.stopBelongsToTrip2(ctx, trip.ID, params.DestinationStopID); err != nil || !ok {
			if err != nil {
				return nil, mcpItem{}, err
			}
			return nil, mcpItem{}, errors.New("destinationAreaId does not belong to this trip")
		}
		var layer store.ItineraryLayer
		if in.Layer != "" {
			layer, err = api.mcpResolveLayer(ctx, trip.ID, in.Layer)
			if err != nil {
				return nil, mcpItem{}, err
			}
			params.LayerID = layer.ID
		} else {
			layer, err = api.trips.LayerByID(ctx, trip.ID, current.LayerID)
			if err != nil {
				return nil, mcpItem{}, err
			}
		}
		updated, err := api.trips.UpdateItem(ctx, trip.ID, itemID, params)
		if err != nil {
			return nil, mcpItem{}, err
		}
		return nil, toMCPItem(updated, map[uuid.UUID]string{layer.ID: layer.Name}), nil
	})
}

// stopBelongsToTrip2 is stopBelongsToTrip without the *http.Request.
func (api *tripsAPI) stopBelongsToTrip2(ctx context.Context, tripID uuid.UUID, stopID *uuid.UUID) (bool, error) {
	if stopID == nil {
		return true, nil
	}
	_, err := api.trips.StopByID(ctx, tripID, *stopID)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// mcpResolveLayer maps a layer name to the trip's layer: "" means Main,
// known names match case-insensitively, unknown names are created.
func (api *tripsAPI) mcpResolveLayer(ctx context.Context, tripID uuid.UUID, name string) (store.ItineraryLayer, error) {
	if strings.TrimSpace(name) == "" {
		return api.trips.EnsureMainLayer(ctx, tripID)
	}
	layers, err := api.trips.ListLayers(ctx, tripID)
	if err != nil {
		return store.ItineraryLayer{}, err
	}
	owned := 0
	for _, l := range layers {
		if strings.EqualFold(l.Name, name) {
			return l, nil
		}
		if l.OwnerID != nil {
			owned++
		}
	}
	user, _ := auth.UserFrom(ctx)
	return api.trips.CreateLayer(ctx, tripID, user.ID, strings.TrimSpace(name), proposalPalette[owned%len(proposalPalette)])
}
