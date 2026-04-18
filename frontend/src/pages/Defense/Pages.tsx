import "@mantine/core/styles.css";
import { Box, Group, Stack, Text, Title } from "@mantine/core";
import { useEffect, useRef, useState } from "react";
import '../../index.css';
import LobbyChat from "../../components/LobbyChat";
import { useGame } from "../../context/GameContext";

const DEFENSE_SECONDS = 10;

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

export default function Defense() {
  const { nominee } = useGame();
  const [secondsLeft, setSecondsLeft] = useState(DEFENSE_SECONDS);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    intervalRef.current = setInterval(() => {
      setSecondsLeft(prev => (prev <= 1 ? 0 : prev - 1));
    }, 1000);
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, []);

  return (
    <div className="bg-mafiaBlack-default min-h-screen p-4">
      <Stack gap="md" maw={1100} mx="auto">
        <Group justify="space-between" wrap="nowrap">
          <Title order={2} style={{ color: "#E94560", letterSpacing: "0.1em" }}>
            DEFENSE
          </Title>
          <Text c="#3E8E7E" fw={700}>
            0:{secondsLeft.toString().padStart(2, "0")}
          </Text>
        </Group>

        <Box p="lg" style={panelRed}>
          <Stack gap="xs">
            <Text size="sm" c="dimmed" tt="uppercase" fw={700}>
              On trial
            </Text>
            <Text size="xl" c="white" fw={700}>
              {nominee?.name ?? "Unknown"}
            </Text>
            <Text size="sm" c="dimmed">
              {nominee?.name ?? "The nominee"} has {DEFENSE_SECONDS}s to defend themselves before the town votes.
            </Text>
          </Stack>
        </Box>

        <Box p="lg" style={panelTeal}>
          <Stack gap="sm">
            <Text size="sm" c="dimmed" tt="uppercase" fw={700}>
              Defense
            </Text>
            <LobbyChat />
          </Stack>
        </Box>
      </Stack>
    </div>
  );
}
