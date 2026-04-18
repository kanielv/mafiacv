import "@mantine/core/styles.css";
import { Box, Group, Loader, ScrollArea, Stack, Text, Title } from "@mantine/core";
import '../../index.css';
import { useGame } from "../../context/GameContext";
import TypewriterText from "../../components/TypewriterText";

const panelRed = {
  border: "4px solid #E94560",
  borderRadius: "0.5rem",
  backgroundColor: "rgba(29,31,39,0.85)",
} as const;

export default function VoteRecap() {
  const { narration, round, voteResult, isHost, endVoteRecap } = useGame();

  const story =
    narration?.storyType === "vote_recap" && narration.round === round
      ? narration.story
      : "";

  const headline = voteResult
    ? voteResult.eliminated
      ? `${voteResult.nomineeName} has been voted out.`
      : `${voteResult.nomineeName} survives the vote.`
    : "";

  const handleDone = () => {
    if (isHost) endVoteRecap();
  };

  return (
    <div className="bg-mafiaBlack-default min-h-screen p-4">
      <Stack gap="md" maw={1100} mx="auto">
        <Group justify="space-between" wrap="nowrap">
          <Title order={2} style={{ color: "#E94560", letterSpacing: "0.1em" }}>
            VERDICT
          </Title>
          {voteResult && (
            <Text c="#3E8E7E" fw={700}>
              {voteResult.yesVotes} YES · {voteResult.noVotes} NO
            </Text>
          )}
        </Group>

        {headline && (
          <Text size="xl" c="white" fw={700}>
            {headline}
          </Text>
        )}

        <Box p="lg" style={panelRed}>
          <Stack gap="sm">
            <Text size="sm" c="dimmed" tt="uppercase" fw={700}>
              Vote Recap
            </Text>
            <ScrollArea h={420}>
              {story ? (
                <TypewriterText
                  text={story}
                  size="lg"
                  c="white"
                  onDone={handleDone}
                />
              ) : (
                <Group gap="sm">
                  <Loader size="sm" color="#E94560" />
                  <Text c="dimmed">The narrator is preparing the verdict...</Text>
                </Group>
              )}
            </ScrollArea>
          </Stack>
        </Box>
      </Stack>
    </div>
  );
}
