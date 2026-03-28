import { useEffect, useState } from "react";
import {
  MantineProvider,
  Button,
  TextInput,
  Title,
  List,
  Text,
  Container,
} from "@mantine/core";
import { theme } from "../../theme";
import RoleSelection from "./RoleSelection";
import LobbyChat from "../../components/LobbyChat";
import { Player } from "../../models/player";
import GoogleTTS from "../../GoogleTTS";
import { sendEvent, onEvent, offEvent, getSocketId } from "../../socket";

export default function Home() {
  const [lobbyId, setLobbyId] = useState<string | null>(null);
  const [isHost, setIsHost] = useState(false);
  const [players, setPlayers] = useState<Player[]>([]);
  const [inputLobbyId, setInputLobbyId] = useState("");
  const [role, setRole] = useState("");
  const [name, setName] = useState("");
  const [story, setStory] = useState("");
  const [roleConfig, setRoleConfig] = useState<Record<string, number>>({
    mafia: 0,
    medic: 0,
    sheriff: 0,
    jester: 0,
  });

  useEffect(() => {
    const onConnected = (data: { socketId: string }) => {
      console.log("Connected to server:", data.socketId);
    };

    const onLobbyCreated = (data: { lobbyId: string; players: Player[] }) => {
      setLobbyId(data.lobbyId);
      setIsHost(true);
      setPlayers(data.players);
    };

    const onPlayersUpdated = (data: { players: Player[] }) => {
      setPlayers(data.players);
    };

    const onGameStarted = () => {
      alert("The game has started!");
    };

    const onUserDisconnected = (data: { socketId: string }) => {
      setPlayers((prevPlayers) =>
        prevPlayers.filter((player) => player.socketID !== data.socketId)
      );
      console.log(`Player with ID ${data.socketId} has disconnected.`);
    };

    const onRolesAssigned = (data: { role: string }) => {
      console.log("Role assigned:", data.role);
      setRole(data.role);
    };

    onEvent("connected", onConnected);
    onEvent("lobby-created", onLobbyCreated);
    onEvent("players-updated", onPlayersUpdated);
    onEvent("game-started", onGameStarted);
    onEvent("user-disconnected", onUserDisconnected);
    onEvent("roles-assigned", onRolesAssigned);

    return () => {
      offEvent("connected", onConnected);
      offEvent("lobby-created", onLobbyCreated);
      offEvent("players-updated", onPlayersUpdated);
      offEvent("game-started", onGameStarted);
      offEvent("user-disconnected", onUserDisconnected);
      offEvent("roles-assigned", onRolesAssigned);
    };
  }, []);

  const createLobby = () => {
    if (name) {
      sendEvent("create-lobby", { name });
    } else {
      console.log("Name is required to create a lobby");
    }
  };

  const joinLobby = () => {
    if (inputLobbyId) {
      sendEvent("join-lobby", { lobbyId: inputLobbyId, name });
      setLobbyId(inputLobbyId);
      console.log("Joining lobby:", inputLobbyId);
    }
  };

  const startGame = () => {
    if (lobbyId) {
      sendEvent("start-game", { lobbyId, roles: roleConfig });
      console.log("Game started in lobby:", lobbyId);
    }
  };

  const generateStory = () => {
    sendEvent("generate-story", {
      code: "reirere",
      names: ["ryder", "wilson", "lazzy"],
      victim: "lazzy",
      killer: "ryder",
      location: "the beach",
    });
    console.log("story created!");
  };

  return (
    <MantineProvider theme={theme}>
      <div className="home">
        <div style={{ padding: "20px" }}>
          <Title order={2}>
            {lobbyId ? `Lobby ID: ${lobbyId}` : "Welcome to the Game!"}
          </Title>

          {lobbyId ? (
            <div>
              {isHost ? (
                <>
                  <Button
                    onClick={startGame}
                    color="blue"
                    style={{ marginBottom: "20px" }}
                  >
                    Start Game
                  </Button>
                  <RoleSelection lobbyId={lobbyId} playerCount={players.length} onChange={setRoleConfig} />
                </>
              ) : (
                <Text size="lg">Waiting for host to start the game...</Text>
              )}
              <Text size="lg">Players:</Text>
              <List>
                {players.map((player) => (
                  <List.Item key={player.socketID}>{player.name}</List.Item>
                ))}
              </List>
              <LobbyChat lobbyId={lobbyId} />
              <Title>{role}</Title>
            </div>
          ) : (
            <div>
              <Button
                onClick={createLobby}
                color="green"
                style={{ marginBottom: "20px" }}
              >
                Host Game
              </Button>
              <TextInput
                placeholder="Enter Lobby ID"
                value={inputLobbyId}
                onChange={(e) => setInputLobbyId(e.currentTarget.value)}
                style={{ marginBottom: "20px" }}
              />
              <TextInput
                label="Name"
                placeholder="Enter your name"
                value={name}
                onChange={(event) => {
                  setName(event.currentTarget.value);
                }}
              />

              <Button
                onClick={() => {
                  if (name && inputLobbyId) {
                    joinLobby();
                  } else {
                    console.log("Name and join code are required");
                  }
                }}
                color="green"
              >
                Join Game
              </Button>

              <Button
                onClick={() => {
                  generateStory();
                }}
                color="green"
                style={{ marginLeft: "20px" }}
              >
                Generate Story
              </Button>

              <GoogleTTS placeholderText={story} />
            </div>
          )}
        </div>
      </div>
    </MantineProvider>
  );
}
