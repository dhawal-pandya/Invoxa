# Invoxa

Invoxa is a simple, multi-tenant invoicing application built with Go and Gin. It provides a basic API for managing organizations, users, subscriptions, and invoices.

## Features

*   **Multi-tenancy:** The application supports multiple organizations, each with its own users, subscriptions, and invoices.
*   **User Management:** Users can be created and assigned to organizations.
*   **Subscription Management:** Organizations can subscribe to different subscription plans.
*   **Invoice Management:** Invoices are automatically generated for subscriptions and can be paid.
*   **Billing Management:** The application includes basic billing features, such as prorated billing for plan upgrades.

## Getting Started

### Prerequisites

*   Go 1.18 or later
*   PostgreSQL

### Installation

1.  Clone the repository:

    ```
    git clone https://github.com/your-username/invoxa.git
    ```

2.  Install the dependencies:

    ```
    go mod tidy
    ```

3.  Create a PostgreSQL database and update the connection string in `database/database.go`.

4.  Run the application:

    ```
    go run main.go
    ```

The application will be available at `http://localhost:8080`.

## API Endpoints

The following API endpoints are available:

*   `POST /organizations`: Create a new organization.
*   `GET /org/:id/summary`: Get a summary of an organization's data.
*   `POST /users`: Create a new user.
*   `GET /user/:id/subscriptions`: Get a user's subscriptions.
*   `POST /subscribe`: Subscribe an organization to a subscription plan.
*   `POST /pay_invoice`: Pay an invoice.
*   `POST /upgrade_plan`: Upgrade an organization's subscription plan.
*   `GET /invoice/:id`: Get an invoice.
*   `POST /refund`: Refund a payment.
*   `POST /subscription_plans`: Create a new subscription plan.
*   `POST /admin/clear_db`: Clear the database.
*   `GET /ping`: Check if the application is running.

## Testing

To run the tests, run the following command:

```
go test ./...
```
