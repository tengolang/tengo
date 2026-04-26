n = 20000
text = "lorem ipsum dolor sit amet consectetur adipiscing elit " * n

words = text.split(" ")
counts = {}

for w in words:
    if w == "":
        continue
    if w not in counts:
        counts[w] = 1
    else:
        counts[w] += 1

out = 0
for k in counts:
    out += counts[k]

print(out)
