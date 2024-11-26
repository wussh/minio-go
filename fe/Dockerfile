# Use the official Nginx image to serve the frontend
FROM nginx:alpine

# Copy the frontend code (HTML and JS) to Nginx's default HTML directory
COPY ./ /usr/share/nginx/html

# Expose port 80 to serve the frontend
EXPOSE 80

# Nginx will automatically start on container run
CMD ["nginx", "-g", "daemon off;"]
