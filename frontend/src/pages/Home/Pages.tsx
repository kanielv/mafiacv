import { useState } from "react";
import {
  Box,
  Button,
  Stack,
  TextInput,
  Title,
  Text,
  Tooltip,
  Divider,
} from "@mantine/core";
import { useGame } from "../../context/GameContext";

const darkInputStyles = {
  input: {
    backgroundColor: "#1D1F27",
    color: "white",
    borderColor: "#3E8E7E",
    borderWidth: "2px",
  },
  label: { color: "white" },
} as const;

export default function Home() {
  const { name, setName, createLobby, joinLobby } = useGame();
  const [inputLobbyId, setInputLobbyId] = useState("");

  const needsName = name.trim().length === 0;
  const canJoin = !needsName && inputLobbyId.trim().length > 0;

  return (
    <div className="bg-mafiaBlack-default min-h-screen flex items-center justify-center p-4">
      <Stack align="center" gap="lg" style={{ width: "100%", maxWidth: 460 }}>
        <Stack align="center" gap={4}>
          <Title order={1} style={{ color: "#E94560", letterSpacing: "0.2em" }}>
            MAFIA
          </Title>
          <Text c="dimmed" size="sm">
            a social deduction tale
          </Text>
        </Stack>

        <Box
          p="lg"
          style={{
            border: "4px solid #E94560",
            borderRadius: "0.5rem",
            backgroundColor: "rgba(29,31,39,0.85)",
            width: "100%",
          }}
        >
          <Stack gap="md">
            <TextInput
              label="Name"
              placeholder="Enter your name"
              value={name}
              onChange={(e) => setName(e.currentTarget.value)}
              styles={darkInputStyles}
            />

            <Tooltip
              label="Enter a name first"
              disabled={!needsName}
              withArrow
            >
              <Button
                fullWidth
                size="md"
                variant="outline"
                color="red"
                disabled={needsName}
                onClick={createLobby}
                styles={{ root: { borderWidth: "4px" } }}
              >
                Host Game
              </Button>
            </Tooltip>

            <Divider
              label={<Text c="dimmed">— or —</Text>}
              labelPosition="center"
              color="#3E8E7E"
            />

            <TextInput
              label="Lobby Code"
              placeholder="e.g. ABC123"
              value={inputLobbyId}
              onChange={(e) => setInputLobbyId(e.currentTarget.value)}
              styles={darkInputStyles}
            />

            <Tooltip
              label={needsName ? "Enter a name first" : "Enter a lobby code"}
              disabled={canJoin}
              withArrow
            >
              <Button
                fullWidth
                size="md"
                variant="outline"
                color="teal"
                disabled={!canJoin}
                onClick={() => joinLobby(inputLobbyId.trim())}
                styles={{
                  root: { borderWidth: "4px", borderColor: "#3E8E7E", color: "#3E8E7E" },
                }}
              >
                Join Game
              </Button>
            </Tooltip>
          </Stack>
        </Box>
      </Stack>
    </div>
  );
}
