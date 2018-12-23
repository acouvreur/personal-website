import express from 'express'
import helmet from 'helmet'
import ejs from 'ejs'

const app = express();

const port = process.env.PORT || 1337

app.use(helmet())
app.use(express.static(__dirname + '/public'));

const server = app.listen(port, () => {
    console.log(`Express running → PORT ${server.address().port}`)
})

app.get('/', (req, res) => {
    res.render(__dirname + '/pages/index.ejs', {title: "Alexis Couvreur"})
})

// Must be last
// Handle 404 not found
app.get('*', (req, res) => {
    res.render(__dirname + '/pages/404.ejs', {title: "Whoops..."})
})