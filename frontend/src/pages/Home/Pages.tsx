import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
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
import { useGame } from "../../context/GameContext";

export default function Home() {
  const { name, setName, lobbyId, isHost, createLobby, joinLobby, startGame, players, setRoleConfig, role } = useGame();
  const [inputLobbyId, setInputLobbyId] = useState("");
  const [story, setStory] = useState("");
  
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
                    joinLobby(inputLobbyId);
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
