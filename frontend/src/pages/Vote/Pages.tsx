import "@mantine/core/styles.css";
import { Box, Button, Group, Stack, Text, Title } from "@mantine/core";
import { useEffect, useRef, useState } from "react";
import '../../index.css';
import { useGame } from "../../context/GameContext";
import { getSocketId } from "../../socket";

const VOTE_SECONDS = 10;

const panelRed = {
  border: "4px solid #E94560",
  borderRadius: "0.5rem",
  backgroundColor: "rgba(29,31,39,0.85)",
} as const;

export default function Vote() {
  const { nominee, voteSubmitted, submitDayVote, players } = useGame();
  const [secondsLeft, setSecondsLeft] = useState(VOTE_SECONDS);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    intervalRef.current = setInterval(() => {
      setSecondsLeft(prev => (prev <= 1 ? 0 : prev - 1));
    }, 1000);
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, []);

  const socketId = getSocketId();
  const me = players.find(p => p.socketID === socketId);
  const isNominee = nominee?.id === socketId;
  const canVote = !!me?.isAlive && !isNominee && !voteSubmitted;

  return (
    <div className="bg-mafiaBlack-default min-h-screen p-4">
      <Stack gap="md" maw={900} mx="auto">
        <Group justify="space-between" wrap="nowrap">
          <Title order={2} style={{ color: "#E94560", letterSpacing: "0.1em" }}>
            VOTE
          </Title>
          <Text c="#3E8E7E" fw={700}>
            0:{secondsLeft.toString().padStart(2, "0")}
          </Text>
        </Group>

        <Box p="lg" style={panelRed}>
          <Stack gap="md" align="center">
            <Text size="xl" c="white" fw={700}>
              Eliminate {nominee?.name ?? "the nominee"}?
            </Text>

            {isNominee && (
              <Text size="sm" c="dimmed">You cannot vote on your own elimination.</Text>
            )}
            {!me?.isAlive && (
              <Text size="sm" c="dimmed">Dead players cannot vote.</Text>
            )}
            {voteSubmitted && (
              <Text size="sm" c="#3E8E7E">Vote cast.</Text>
            )}

            <Group justify="center" gap="lg">
              <Button
                size="xl"
                color="red"
                disabled={!canVote}
                onClick={() => submitDayVote(true)}
              >
                YES — eliminate
              </Button>
              <Button
                size="xl"
                color="teal"
                disabled={!canVote}
                onClick={() => submitDayVote(false)}
              >
                NO — spare
              </Button>
            </Group>
          </Stack>
        </Box>
      </Stack>
    </div>
  );
}
