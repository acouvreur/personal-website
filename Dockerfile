FROM mhart/alpine-node:10

WORKDIR /app
COPY . .

RUN npm install

EXPOSE 1337
CMD ["node", "start.js"]