cgpt --help  # What flags are actually available?

# Maybe start with just two commands:
echo "What capabilities do you have?" | cgpt -s "you are node1"
echo "Here's what node1 said: $(cat previous_output)" | cgpt -s "you are node2"
