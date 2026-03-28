import "@mantine/core/styles.css";
import { Group, Stack, Text, Image, ScrollArea, Button } from "@mantine/core";
import { useState, useEffect } from "react";
import '../../index.css';
import GoogleTTS from "../../GoogleTTS";
import { sendEvent, onEvent, offEvent } from "../../socket";


export default function Layout() {
  const [story, setStory] = useState("");

  useEffect(() => {
    const onConnected = (data: { socketId: string }) => {
      console.log("Connected to server:", data.socketId);
    };

    const onStoryGenerated = (data: { story: string }) => {
      setStory(data.story);
    };

    onEvent("connected", onConnected);
    onEvent("story-generated", onStoryGenerated);

    return () => {
      offEvent("connected", onConnected);
      offEvent("story-generated", onStoryGenerated);
    };
  }, []);

  // generate story via socket listener
  const generateStory = () => {
    sendEvent("generate-story", {
      code: "exampleCode",
      names: ["ryder", "wilson", "lazzy"],
      victim: "lazzy",
      killer: "ryder",
      location: "school",
    });
  }

  return (

    <div className="bg-mafiaBlack-default min-h-screen items-center">
      <Group justify="center" align="center" gap={50} className="flex flex-col min-h-screen md:flex-row gap-4 w-full border-4 border-blue-500">
        <Stack align="center" justify="center" className="w-full md:w-1/3">
          <Stack align="center" justify="center" className="w-full border-4 border-blue-500">
            <Text size="lg" c="white" mt="xs">Theme: L'Checkers</Text>
            <Image radius="md" mb="xs" mr="xs" ml="xs" src="https://raw.githubusercontent.com/mantinedev/mantine/master/.demo/images/bg-7.png" className="border-4 border-mafiaRed-default" />
          </Stack>
          <Group>
            <GoogleTTS placeholderText={story} />
            <Button
              onClick={() => {generateStory();}}
              color="green"
              style={{marginLeft: "20px"}}
            >
              Generate Story
            </Button>
          </Group>
        </Stack>
        <Stack mt="sm" className="w-full md:w-1/3 border-4 border-mafiaRed-default rounded-md">
          <Group justify="center" align="center">
              <div className="min-w-[100%] w-[100%] bg-mafiaBlack-default p-4">
                <ScrollArea h={400}>
                  <Text size="xl" c="white">{story}</Text>
                </ScrollArea>
              </div>
          </Group>
        </Stack>
      </Group>
    </div>
  );
}
