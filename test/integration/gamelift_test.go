//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/gamelift"
	"github.com/sivchari/golden"
)

func newGameLiftClient(t *testing.T) *gamelift.Client {
	t.Helper()

	return gamelift.NewFromConfig(awsConfig(t), func(o *gamelift.Options) {
		o.BaseEndpoint = aws.String(testEndpoint())
	})
}

func TestGameLift_CreateAndDeleteBuild(t *testing.T) {
	client := newGameLiftClient(t)
	ctx := t.Context()

	buildName := "test-build"
	buildVersion := "1.0.0"

	// Create build.
	createOutput, err := client.CreateBuild(ctx, &gamelift.CreateBuildInput{
		Name:    aws.String(buildName),
		Version: aws.String(buildVersion),
	})
	if err != nil {
		t.Fatal(err)
	}

	buildID := createOutput.Build.BuildId

	t.Cleanup(func() {
		_, _ = client.DeleteBuild(context.Background(), &gamelift.DeleteBuildInput{
			BuildId: buildID,
		})
	})

	g := golden.New(t,
		golden.WithIgnoreFields("BuildId", "BuildArn", "CreationTime", "UploadCredentials", "StorageLocation"),
	)
	g.Assert(t.Name()+"/CreateBuild", createOutput)

	// Describe build.
	descOutput, err := client.DescribeBuild(ctx, &gamelift.DescribeBuildInput{
		BuildId: buildID,
	})
	if err != nil {
		t.Fatal(err)
	}
	g.Assert(t.Name()+"/DescribeBuild", descOutput)

	// Delete build.
	_, err = client.DeleteBuild(ctx, &gamelift.DeleteBuildInput{
		BuildId: buildID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify build is deleted.
	_, err = client.DescribeBuild(ctx, &gamelift.DescribeBuildInput{
		BuildId: buildID,
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestGameLift_ListBuilds(t *testing.T) {
	client := newGameLiftClient(t)
	ctx := t.Context()

	// Create multiple builds.
	var buildIDs []*string

	for i := 0; i < 3; i++ {
		createOutput, err := client.CreateBuild(ctx, &gamelift.CreateBuildInput{
			Name:    aws.String("test-build-list"),
			Version: aws.String("1.0.0"),
		})
		if err != nil {
			t.Fatal(err)
		}
		buildIDs = append(buildIDs, createOutput.Build.BuildId)
	}

	t.Cleanup(func() {
		for _, buildID := range buildIDs {
			_, _ = client.DeleteBuild(context.Background(), &gamelift.DeleteBuildInput{
				BuildId: buildID,
			})
		}
	})

	// List builds.
	listOutput, err := client.ListBuilds(ctx, &gamelift.ListBuildsInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listOutput.Builds) < 3 {
		t.Errorf("expected at least 3 builds, got %d", len(listOutput.Builds))
	}
}

func TestGameLift_CreateAndDeleteFleet(t *testing.T) {
	client := newGameLiftClient(t)
	ctx := t.Context()

	// Create a build first.
	buildOutput, err := client.CreateBuild(ctx, &gamelift.CreateBuildInput{
		Name:    aws.String("test-build-for-fleet"),
		Version: aws.String("1.0.0"),
	})
	if err != nil {
		t.Fatal(err)
	}

	buildID := buildOutput.Build.BuildId

	t.Cleanup(func() {
		_, _ = client.DeleteBuild(context.Background(), &gamelift.DeleteBuildInput{
			BuildId: buildID,
		})
	})

	fleetName := "test-fleet"

	// Create fleet.
	createOutput, err := client.CreateFleet(ctx, &gamelift.CreateFleetInput{
		Name:            aws.String(fleetName),
		BuildId:         buildID,
		EC2InstanceType: "c5.large",
	})
	if err != nil {
		t.Fatal(err)
	}

	fleetID := createOutput.FleetAttributes.FleetId

	t.Cleanup(func() {
		_, _ = client.DeleteFleet(context.Background(), &gamelift.DeleteFleetInput{
			FleetId: fleetID,
		})
	})

	g := golden.New(t,
		golden.WithIgnoreFields("FleetId", "FleetArn", "BuildId", "BuildArn", "CreationTime"),
	)
	g.Assert(t.Name()+"/CreateFleet", createOutput)

	// Describe fleet attributes.
	descOutput, err := client.DescribeFleetAttributes(ctx, &gamelift.DescribeFleetAttributesInput{
		FleetIds: []string{*fleetID},
	})
	if err != nil {
		t.Fatal(err)
	}
	g.Assert(t.Name()+"/DescribeFleetAttributes", descOutput)

	// Delete fleet.
	_, err = client.DeleteFleet(ctx, &gamelift.DeleteFleetInput{
		FleetId: fleetID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify fleet is deleted.
	descOutput, err = client.DescribeFleetAttributes(ctx, &gamelift.DescribeFleetAttributesInput{
		FleetIds: []string{*fleetID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(descOutput.FleetAttributes) != 0 {
		t.Errorf("expected empty fleet attributes, got %d", len(descOutput.FleetAttributes))
	}
}

func TestGameLift_ListFleets(t *testing.T) {
	client := newGameLiftClient(t)
	ctx := t.Context()

	// Create a build first.
	buildOutput, err := client.CreateBuild(ctx, &gamelift.CreateBuildInput{
		Name:    aws.String("test-build-for-list-fleets"),
		Version: aws.String("1.0.0"),
	})
	if err != nil {
		t.Fatal(err)
	}

	buildID := buildOutput.Build.BuildId

	t.Cleanup(func() {
		_, _ = client.DeleteBuild(context.Background(), &gamelift.DeleteBuildInput{
			BuildId: buildID,
		})
	})

	// Create multiple fleets.
	var fleetIDs []*string

	for i := 0; i < 2; i++ {
		createOutput, err := client.CreateFleet(ctx, &gamelift.CreateFleetInput{
			Name:            aws.String("test-fleet-list"),
			BuildId:         buildID,
			EC2InstanceType: "c5.large",
		})
		if err != nil {
			t.Fatal(err)
		}
		fleetIDs = append(fleetIDs, createOutput.FleetAttributes.FleetId)
	}

	t.Cleanup(func() {
		for _, fleetID := range fleetIDs {
			_, _ = client.DeleteFleet(context.Background(), &gamelift.DeleteFleetInput{
				FleetId: fleetID,
			})
		}
	})

	// List fleets.
	listOutput, err := client.ListFleets(ctx, &gamelift.ListFleetsInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listOutput.FleetIds) < 2 {
		t.Errorf("expected at least 2 fleets, got %d", len(listOutput.FleetIds))
	}
}

func TestGameLift_CreateAndDescribeGameSession(t *testing.T) {
	client := newGameLiftClient(t)
	ctx := t.Context()

	// Create a build.
	buildOutput, err := client.CreateBuild(ctx, &gamelift.CreateBuildInput{
		Name:    aws.String("test-build-for-gamesession"),
		Version: aws.String("1.0.0"),
	})
	if err != nil {
		t.Fatal(err)
	}

	buildID := buildOutput.Build.BuildId

	t.Cleanup(func() {
		_, _ = client.DeleteBuild(context.Background(), &gamelift.DeleteBuildInput{
			BuildId: buildID,
		})
	})

	// Create a fleet.
	fleetOutput, err := client.CreateFleet(ctx, &gamelift.CreateFleetInput{
		Name:            aws.String("test-fleet-for-gamesession"),
		BuildId:         buildID,
		EC2InstanceType: "c5.large",
	})
	if err != nil {
		t.Fatal(err)
	}

	fleetID := fleetOutput.FleetAttributes.FleetId

	t.Cleanup(func() {
		_, _ = client.DeleteFleet(context.Background(), &gamelift.DeleteFleetInput{
			FleetId: fleetID,
		})
	})

	// Create a game session.
	sessionName := "test-game-session"
	maxPlayers := int32(10)

	createSessionOutput, err := client.CreateGameSession(ctx, &gamelift.CreateGameSessionInput{
		FleetId:                   fleetID,
		Name:                      aws.String(sessionName),
		MaximumPlayerSessionCount: aws.Int32(maxPlayers),
	})
	if err != nil {
		t.Fatal(err)
	}

	gameSessionID := createSessionOutput.GameSession.GameSessionId

	g := golden.New(t,
		golden.WithIgnoreFields("GameSessionId", "FleetId", "FleetArn", "CreationTime", "TerminationTime", "IpAddress", "DnsName", "Port"),
	)
	g.Assert(t.Name()+"/CreateGameSession", createSessionOutput)

	// Describe game session.
	descOutput, err := client.DescribeGameSessions(ctx, &gamelift.DescribeGameSessionsInput{
		GameSessionId: gameSessionID,
	})
	if err != nil {
		t.Fatal(err)
	}
	g.Assert(t.Name()+"/DescribeGameSessions", descOutput)
}

func TestGameLift_UpdateGameSession(t *testing.T) {
	client := newGameLiftClient(t)
	ctx := t.Context()

	// Create a build.
	buildOutput, err := client.CreateBuild(ctx, &gamelift.CreateBuildInput{
		Name:    aws.String("test-build-for-update"),
		Version: aws.String("1.0.0"),
	})
	if err != nil {
		t.Fatal(err)
	}

	buildID := buildOutput.Build.BuildId

	t.Cleanup(func() {
		_, _ = client.DeleteBuild(context.Background(), &gamelift.DeleteBuildInput{
			BuildId: buildID,
		})
	})

	// Create a fleet.
	fleetOutput, err := client.CreateFleet(ctx, &gamelift.CreateFleetInput{
		Name:            aws.String("test-fleet-for-update"),
		BuildId:         buildID,
		EC2InstanceType: "c5.large",
	})
	if err != nil {
		t.Fatal(err)
	}

	fleetID := fleetOutput.FleetAttributes.FleetId

	t.Cleanup(func() {
		_, _ = client.DeleteFleet(context.Background(), &gamelift.DeleteFleetInput{
			FleetId: fleetID,
		})
	})

	// Create a game session.
	createSessionOutput, err := client.CreateGameSession(ctx, &gamelift.CreateGameSessionInput{
		FleetId:                   fleetID,
		Name:                      aws.String("original-name"),
		MaximumPlayerSessionCount: aws.Int32(10),
	})
	if err != nil {
		t.Fatal(err)
	}

	gameSessionID := createSessionOutput.GameSession.GameSessionId

	// Update game session.
	newName := "updated-name"
	newMaxPlayers := int32(20)

	updateOutput, err := client.UpdateGameSession(ctx, &gamelift.UpdateGameSessionInput{
		GameSessionId:             gameSessionID,
		Name:                      aws.String(newName),
		MaximumPlayerSessionCount: aws.Int32(newMaxPlayers),
	})
	if err != nil {
		t.Fatal(err)
	}

	g := golden.New(t,
		golden.WithIgnoreFields("GameSessionId", "FleetId", "FleetArn", "CreationTime", "TerminationTime", "IpAddress", "DnsName", "Port"),
	)
	g.Assert(t.Name(), updateOutput)
}

func TestGameLift_CreateAndDescribePlayerSession(t *testing.T) {
	client := newGameLiftClient(t)
	ctx := t.Context()

	// Create a build.
	buildOutput, err := client.CreateBuild(ctx, &gamelift.CreateBuildInput{
		Name:    aws.String("test-build-for-playersession"),
		Version: aws.String("1.0.0"),
	})
	if err != nil {
		t.Fatal(err)
	}

	buildID := buildOutput.Build.BuildId

	t.Cleanup(func() {
		_, _ = client.DeleteBuild(context.Background(), &gamelift.DeleteBuildInput{
			BuildId: buildID,
		})
	})

	// Create a fleet.
	fleetOutput, err := client.CreateFleet(ctx, &gamelift.CreateFleetInput{
		Name:            aws.String("test-fleet-for-playersession"),
		BuildId:         buildID,
		EC2InstanceType: "c5.large",
	})
	if err != nil {
		t.Fatal(err)
	}

	fleetID := fleetOutput.FleetAttributes.FleetId

	t.Cleanup(func() {
		_, _ = client.DeleteFleet(context.Background(), &gamelift.DeleteFleetInput{
			FleetId: fleetID,
		})
	})

	// Create a game session.
	createSessionOutput, err := client.CreateGameSession(ctx, &gamelift.CreateGameSessionInput{
		FleetId:                   fleetID,
		Name:                      aws.String("test-session"),
		MaximumPlayerSessionCount: aws.Int32(10),
	})
	if err != nil {
		t.Fatal(err)
	}

	gameSessionID := createSessionOutput.GameSession.GameSessionId

	// Create a player session.
	playerID := "player-123"

	createPlayerOutput, err := client.CreatePlayerSession(ctx, &gamelift.CreatePlayerSessionInput{
		GameSessionId: gameSessionID,
		PlayerId:      aws.String(playerID),
	})
	if err != nil {
		t.Fatal(err)
	}

	playerSessionID := createPlayerOutput.PlayerSession.PlayerSessionId

	g := golden.New(t,
		golden.WithIgnoreFields("PlayerSessionId", "GameSessionId", "FleetId", "FleetArn", "CreationTime", "TerminationTime", "IpAddress", "DnsName", "Port"),
	)
	g.Assert(t.Name()+"/CreatePlayerSession", createPlayerOutput)

	// Describe player session.
	descOutput, err := client.DescribePlayerSessions(ctx, &gamelift.DescribePlayerSessionsInput{
		PlayerSessionId: playerSessionID,
	})
	if err != nil {
		t.Fatal(err)
	}
	g.Assert(t.Name()+"/DescribePlayerSessions", descOutput)
}

func TestGameLift_CreatePlayerSessions(t *testing.T) {
	client := newGameLiftClient(t)
	ctx := t.Context()

	// Create a build.
	buildOutput, err := client.CreateBuild(ctx, &gamelift.CreateBuildInput{
		Name:    aws.String("test-build-for-playersessions"),
		Version: aws.String("1.0.0"),
	})
	if err != nil {
		t.Fatal(err)
	}

	buildID := buildOutput.Build.BuildId

	t.Cleanup(func() {
		_, _ = client.DeleteBuild(context.Background(), &gamelift.DeleteBuildInput{
			BuildId: buildID,
		})
	})

	// Create a fleet.
	fleetOutput, err := client.CreateFleet(ctx, &gamelift.CreateFleetInput{
		Name:            aws.String("test-fleet-for-playersessions"),
		BuildId:         buildID,
		EC2InstanceType: "c5.large",
	})
	if err != nil {
		t.Fatal(err)
	}

	fleetID := fleetOutput.FleetAttributes.FleetId

	t.Cleanup(func() {
		_, _ = client.DeleteFleet(context.Background(), &gamelift.DeleteFleetInput{
			FleetId: fleetID,
		})
	})

	// Create a game session.
	createSessionOutput, err := client.CreateGameSession(ctx, &gamelift.CreateGameSessionInput{
		FleetId:                   fleetID,
		Name:                      aws.String("test-session"),
		MaximumPlayerSessionCount: aws.Int32(10),
	})
	if err != nil {
		t.Fatal(err)
	}

	gameSessionID := createSessionOutput.GameSession.GameSessionId

	// Create multiple player sessions.
	playerIDs := []string{"player-1", "player-2", "player-3"}

	createPlayersOutput, err := client.CreatePlayerSessions(ctx, &gamelift.CreatePlayerSessionsInput{
		GameSessionId: gameSessionID,
		PlayerIds:     playerIDs,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(createPlayersOutput.PlayerSessions) != 3 {
		t.Errorf("expected 3 player sessions, got %d", len(createPlayersOutput.PlayerSessions))
	}
}

func TestGameLift_BuildNotFound(t *testing.T) {
	client := newGameLiftClient(t)
	ctx := t.Context()

	// Describe non-existent build.
	_, err := client.DescribeBuild(ctx, &gamelift.DescribeBuildInput{
		BuildId: aws.String("non-existent-build"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Delete non-existent build.
	_, err = client.DeleteBuild(ctx, &gamelift.DeleteBuildInput{
		BuildId: aws.String("non-existent-build"),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestGameLift_FleetNotFound(t *testing.T) {
	client := newGameLiftClient(t)
	ctx := t.Context()

	// Delete non-existent fleet.
	_, err := client.DeleteFleet(ctx, &gamelift.DeleteFleetInput{
		FleetId: aws.String("non-existent-fleet"),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestGameLift_GameSessionNotFound(t *testing.T) {
	client := newGameLiftClient(t)
	ctx := t.Context()

	// Update non-existent game session.
	_, err := client.UpdateGameSession(ctx, &gamelift.UpdateGameSessionInput{
		GameSessionId: aws.String("non-existent-session"),
		Name:          aws.String("new-name"),
	})
	if err == nil {
		t.Error("expected error")
	}

	// Create player session with non-existent game session.
	_, err = client.CreatePlayerSession(ctx, &gamelift.CreatePlayerSessionInput{
		GameSessionId: aws.String("non-existent-session"),
		PlayerId:      aws.String("player-1"),
	})
	if err == nil {
		t.Error("expected error")
	}
}
