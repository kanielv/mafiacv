import "@mantine/core/styles.css";
import { useMemo, useState } from 'react';
import { Navigate } from 'react-router-dom';
import { Box, Button, Group, Loader, Stack, Text, Title } from "@mantine/core";
import { useGame } from '../../context/GameContext';
import { getSocketId } from '../../socket';
import VotePlayer from '../../components/votePlayer';
import '../../index.css';

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

type ActorRole = 'mafia' | 'sheriff' | 'medic';

const PROMPTS: Record<ActorRole, { title: string; prompt: string; verb: 'kill' | 'investigate' | 'protect'; label: string }> = {
  mafia:   { title: 'MAFIA',   prompt: 'Choose who dies tonight.',      verb: 'kill',        label: 'Kill' },
  sheriff: { title: 'SHERIFF', prompt: "Whose truth will you uncover?", verb: 'investigate', label: 'Investigate' },
  medic:   { title: 'MEDIC',   prompt: 'Who will you watch over?',      verb: 'protect',     label: 'Protect' },
};

export default function Night() {
  const { role, players, nightSubmitted, sheriffResult, submitNightAction } = useGame();
  const selfId = getSocketId();
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const actorRole: ActorRole | null = useMemo(() => {
    if (role === 'mafia' || role === 'sheriff' || role === 'medic') return role;
    return null;
  }, [role]);

  const targets = useMemo(() => {
    if (!actorRole) return [];
    const alive = players.filter(p => p.isAlive);
    if (actorRole === 'medic') return alive;
    return alive.filter(p => p.socketID !== selfId);
  }, [actorRole, players, selfId]);

  if (!role) return <Navigate to="/" replace />;

  if (!actorRole) {
    return (
      <div className="bg-mafiaBlack-default min-h-screen p-4 flex items-center justify-center">
        <Box p="xl" style={panelRed} maw={600}>
          <Stack align="center" gap="sm">
            <Title order={2} style={{ color: "#E94560" }}>NIGHT</Title>
            <Text size="lg" c="white" ta="center">Sleep. Dawn will come.</Text>
            <Loader color="#E94560" />
          </Stack>
        </Box>
      </div>
    );
  }

  const config = PROMPTS[actorRole];
  const selectedTarget = targets.find(t => t.socketID === selectedId);
  const revealedPlayer = sheriffResult ? players.find(p => p.socketID === sheriffResult.targetId) : null;

  return (
    <div className="bg-mafiaBlack-default min-h-screen p-4 flex items-center justify-center">
      <Stack gap="lg" style={{ width: "100%", maxWidth: 640 }}>
        <Box p="lg" style={panelRed}>
          <Stack gap={4} align="center">
            <Text size="sm" c="dimmed" tt="uppercase" fw={700}>Night falls</Text>
            <Title order={1} style={{ color: "#E94560", letterSpacing: "0.1em" }}>{config.title}</Title>
            <Text c="white">{config.prompt}</Text>
          </Stack>
        </Box>

        {nightSubmitted ? (
          <Box p="lg" style={panelTeal}>
            <Group gap="sm" justify="center">
              <Loader size="sm" color="#3E8E7E" />
              <Text c="white">You have chosen. Waiting for dawn…</Text>
            </Group>
          </Box>
        ) : (
          <Box p="lg" style={panelRed}>
            <Stack gap="xs">
              {targets.map(p => (
                <VotePlayer
                  key={p.socketID}
                  name={p.name}
                  label={selectedId === p.socketID ? 'Selected' : 'Select'}
                  selected={selectedId === p.socketID}
                  onClick={() => setSelectedId(p.socketID)}
                />
              ))}
              <Button
                mt="sm"
                color="red"
                disabled={!selectedTarget}
                onClick={() => selectedTarget && submitNightAction(config.verb, selectedTarget.socketID)}
              >
                Confirm {config.label}
              </Button>
            </Stack>
          </Box>
        )}

        {actorRole === 'sheriff' && sheriffResult && (
          <Box p="lg" style={panelTeal}>
            <Stack gap={4} align="center">
              <Text size="sm" c="dimmed" tt="uppercase" fw={700}>Investigation</Text>
              <Text c="white" size="lg">
                <b style={{ color: '#E94560' }}>{revealedPlayer?.name ?? sheriffResult.targetId}</b>
                {' is '}
                <b style={{ color: '#3E8E7E' }}>{sheriffResult.role.toUpperCase()}</b>
              </Text>
            </Stack>
          </Box>
        )}
      </Stack>
    </div>
  );
}
