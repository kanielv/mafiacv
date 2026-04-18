import { useEffect } from "react";
import { useNavigate } from "react-router-dom";
import {
  Container,
  Grid,
  Paper,
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
import RoleSelection from "../Home/RoleSelection";
import ThemeSelection from "../../components/ThemeSelection";

const MIN_PLAYERS = 3;

export default function Lobby() {
  const navigate = useNavigate();
  const { lobbyId, isHost, players, roleConfig, setRoleConfig, theme, setTheme, startGame } = useGame();

  const roleTotal = Object.values(roleConfig).reduce((a, b) => a + b, 0);
  const startDisabledReason =
    players.length < MIN_PLAYERS
      ? `Need at least ${MIN_PLAYERS} players (have ${players.length}).`
      : roleTotal > players.length
      ? `Configured roles (${roleTotal}) exceed players (${players.length}).`
      : null;

  useEffect(() => {
    if (!lobbyId) navigate("/");
  }, [lobbyId, navigate]);

  if (!lobbyId) return null;

  return (
    <Container size="lg" py="xl">
      <Group justify="space-between" mb="lg">
        <Title order={2}>Lobby: {lobbyId}</Title>
        <Button
          variant="light"
          onClick={() => navigator.clipboard.writeText(lobbyId)}
        >
          Copy Code
        </Button>
      </Group>

      <Grid gutter="md">
        <Grid.Col span={4}>
          <Paper shadow="xs" p="md" radius="md" withBorder>
            <Stack>
              <Title order={4}>Players ({players.length})</Title>
              <ScrollArea h={400}>
                <Stack gap="xs">
                  {players.map((player) => (
                    <Text key={player.socketID}>{player.name}</Text>
                  ))}
                </Stack>
              </ScrollArea>
              {!isHost && (
                <Text c="dimmed" ta="center">
                  Waiting for host to start the game...
                </Text>
              )}
            </Stack>
          </Paper>
        </Grid.Col>

        <Grid.Col span={8}>
          <Stack>
            <Paper shadow="xs" p="md" radius="md" withBorder>
              <Title order={4} mb="sm">
                Chat
              </Title>
              <LobbyChat />
            </Paper>

            {isHost && (
              <Paper shadow="xs" p="md" radius="md" withBorder>
                <Title order={4} mb="sm">
                  Theme
                </Title>
                <ThemeSelection value={theme} onChange={setTheme} />

                <Title order={4} mt="lg" mb="sm">
                  Role Setup
                </Title>
                <RoleSelection
                  lobbyId={lobbyId}
                  playerCount={players.length}
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
                    onClick={startGame}
                    disabled={!!startDisabledReason}
                    data-disabled={!!startDisabledReason || undefined}
                  >
                    Start Game
                  </Button>
                </Tooltip>
              </Paper>
            )}
          </Stack>
        </Grid.Col>
      </Grid>
    </Container>
  );
}
