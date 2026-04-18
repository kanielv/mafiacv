import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import {
  Box,
  Grid,
  Stack,
  Group,
  Title,
  Text,
  Button,
  ScrollArea,
  Tooltip,
} from "@mantine/core";
import { useGame } from "../../context/GameContext";
import LobbyChat from "../../components/LobbyChat";
import LobbyPlayer from "../../components/lobbyPlayer";
import RoleSelection from "../Home/RoleSelection";
import ThemeSelection from "../../components/ThemeSelection";

const MIN_PLAYERS = 3;

const panelRed = {
  border: "4px solid #E94560",
  borderRadius: "0.5rem",
  backgroundColor: "rgba(29,31,39,0.85)",
} as const;

const panelTeal = {
  border: "4px solid #3E8E7E",
  borderRadius: "0.5rem",
  backgroundColor: "rgba(29,31,39,0.85)",
} as const;

export default function Lobby() {
  const navigate = useNavigate();
  const {
    lobbyId,
    isHost,
    players,
    roleConfig,
    setRoleConfig,
    theme,
    setTheme,
    startGame,
  } = useGame();

  useEffect(() => {
    if (!lobbyId) navigate("/");
  }, [lobbyId, navigate]);

  if (!lobbyId) return null;

  const roleTotal = Object.values(roleConfig).reduce((a, b) => a + b, 0);
  const startDisabledReason =
    players.length < MIN_PLAYERS
      ? `Need at least ${MIN_PLAYERS} players (have ${players.length}).`
      : roleTotal > players.length
      ? `Configured roles (${roleTotal}) exceed players (${players.length}).`
      : null;

  return (
    <div className="bg-mafiaBlack-default min-h-screen p-4">
      <Group justify="space-between" mb="lg" wrap="nowrap" maw={1100} mx="auto">
        <Title order={2} style={{ color: "#E94560" }}>
          Lobby: {lobbyId}
        </Title>
        <Button
          variant="outline"
          color="teal"
          styles={{ root: { borderColor: "#3E8E7E", color: "#3E8E7E" } }}
          onClick={() => navigator.clipboard.writeText(lobbyId)}
        >
          Copy Code
        </Button>
      </Group>

      <Grid gutter="md" maw={1100} mx="auto">
        <Grid.Col span={{ base: 12, md: 4 }}>
          <Box p="md" style={panelTeal}>
            <Stack>
              <Title order={4} style={{ color: "#E94560" }}>
                Players ({players.length})
              </Title>
              <ScrollArea h={400}>
                <Stack gap="xs">
                  {players.map((p) => (
                    <LobbyPlayer key={p.socketID} name={p.name} />
                  ))}
                </Stack>
              </ScrollArea>
              {!isHost && (
                <Text c="dimmed" ta="center">
                  Waiting for host to start the game...
                </Text>
              )}
            </Stack>
          </Box>
        </Grid.Col>

        <Grid.Col span={{ base: 12, md: 8 }}>
          <Stack>
            <Box p="md" style={panelTeal}>
              <Title order={4} mb="sm" style={{ color: "#E94560" }}>
                Chat
              </Title>
              <LobbyChat />
            </Box>

            {isHost && (
              <Box p="md" style={panelRed}>
                <Title order={4} mb="sm" style={{ color: "#E94560" }}>
                  Theme
                </Title>
                <ThemeSelection value={theme} onChange={setTheme} />

                <Title order={4} mt="lg" mb="sm" style={{ color: "#E94560" }}>
                  Role Setup
                </Title>
                <RoleSelection
                  lobbyId={lobbyId}
                  playerCount={players.length}
                  value={roleConfig}
                  onChange={setRoleConfig}
                />

                <Tooltip
                  label={startDisabledReason ?? ""}
                  disabled={!startDisabledReason}
                  withArrow
                >
                  <Button
                    fullWidth
                    mt="md"
                    size="md"
                    variant="outline"
                    color="red"
                    onClick={startGame}
                    disabled={!!startDisabledReason}
                    data-disabled={!!startDisabledReason || undefined}
                    styles={{ root: { borderWidth: "4px" } }}
                  >
                    Start Game
                  </Button>
                </Tooltip>
              </Box>
            )}
          </Stack>
        </Grid.Col>
      </Grid>
    </div>
  );
}
