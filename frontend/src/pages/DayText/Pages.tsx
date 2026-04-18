import "@mantine/core/styles.css";
import { Box, Grid, Group, Loader, ScrollArea, Stack, Text, Title } from "@mantine/core";
import { Navigate } from "react-router-dom";
import { useEffect, useRef, useState } from "react";
import '../../index.css';
import { useGame } from "../../context/GameContext";
import TypewriterText from "../../components/TypewriterText";
import LobbyChat from "../../components/LobbyChat";

const DISCUSSION_SECONDS = 10;

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

export default function Day() {
  const { narration, players, round, role, dayStartAlive, isHost, endDiscussion } = useGame();

  const [deathsRevealed, setDeathsRevealed] = useState(false);
  const [discussionActive, setDiscussionActive] = useState(false);
  const [secondsLeft, setSecondsLeft] = useState(DISCUSSION_SECONDS);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const story =
    narration?.storyType === "night_recap" && narration.round === round
      ? narration.story
      : "";

  const startDiscussion = () => {
    setDeathsRevealed(true);
    if (discussionActive) return;
    setDiscussionActive(true);
    if (intervalRef.current) clearInterval(intervalRef.current);
    intervalRef.current = setInterval(() => {
      setSecondsLeft(prev => {
        if (prev <= 1) {
          if (intervalRef.current) {
            clearInterval(intervalRef.current);
            intervalRef.current = null;
          }
          // Host drives the transition; everyone else is navigated by the
          // server's phase-changed broadcast.
          if (isHost) endDiscussion();
          return 0;
        }
        return prev - 1;
      });
    }, 1000);
  };

  useEffect(() => {
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, []);

  if (!role) return <Navigate to="/" replace />;

  return (
    <div className="bg-mafiaBlack-default min-h-screen p-4">
      <Stack gap="md" maw={1100} mx="auto">
        <Group justify="space-between" wrap="nowrap">
          <Title order={2} style={{ color: "#E94560", letterSpacing: "0.1em" }}>
            DAY {round}
          </Title>
          {/* <GoogleTTS placeholderText={story} /> */}
        </Group>

        <Grid gutter="md">
          <Grid.Col span={{ base: 12, md: 8 }}>
            <Box p="lg" style={panelRed}>
              <Stack gap="sm">
                <Text size="sm" c="dimmed" tt="uppercase" fw={700}>
                  Night Recap
                </Text>
                <ScrollArea h={420}>
                  {story ? (
                    <TypewriterText
                      text={story}
                      size="lg"
                      c="white"
                      onDone={startDiscussion}
                    />
                  ) : (
                    <Group gap="sm">
                      <Loader size="sm" color="#E94560" />
                      <Text c="dimmed">The narrator is preparing the tale...</Text>
                    </Group>
                  )}
                </ScrollArea>
              </Stack>
            </Box>
          </Grid.Col>

          <Grid.Col span={{ base: 12, md: 4 }}>
            <Box p="md" style={panelTeal}>
              <Stack gap="xs">
                <Text size="sm" c="dimmed" tt="uppercase" fw={700}>
                  Players
                </Text>
                {players.map(p => {
                  const alive = deathsRevealed
                    ? p.isAlive
                    : dayStartAlive[p.socketID] ?? p.isAlive;
                  return (
                    <Text
                      key={p.socketID}
                      c={alive ? "white" : "dimmed"}
                      td={alive ? undefined : "line-through"}
                    >
                      {p.name}
                    </Text>
                  );
                })}
              </Stack>
            </Box>
          </Grid.Col>
        </Grid>

        {discussionActive && (
          <Box p="lg" style={panelTeal}>
            <Stack gap="sm">
              <Group justify="space-between">
                <Text size="sm" c="dimmed" tt="uppercase" fw={700}>
                  Discussion
                </Text>
                <Text c="#3E8E7E" fw={700}>
                  0:{secondsLeft.toString().padStart(2, "0")}
                </Text>
              </Group>
              <LobbyChat />
            </Stack>
          </Box>
        )}
      </Stack>
    </div>
  );
}
