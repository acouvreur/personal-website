import compression from 'compression'
import express from 'express'
import helmet from 'helmet'
import ejs from 'ejs'

const app = express();

const port = process.env.PORT || 1337

app.use(helmet())
app.use(compression());
app.use(express.static('public'));

const server = app.listen(port, () => {
    console.log(`Express running → PORT ${server.address().port}`)
})

// app.use(serveStatic('pages', {'index': ['default.html']}))