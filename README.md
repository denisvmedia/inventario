# Inventario - Your Personal Inventory

Welcome to Inventario - the ultimate app for managing and organizing your personal inventory.

Note, the project is still under development.

## Future Features

- **Inventory Management**: Easily create, update, and delete items in your inventory. Add essential details such as item name, description, location, purchase date, and more.

- **Categorization and Tags**: Categorize your items into different areas, such as rooms in your house, storage units, or any custom locations you prefer. Assign tags to items for easy filtering and organization.

- **Commodity Tracking**: Track the status of your items, including whether they are in use, sold, lost, disposed of, or written off. Monitor the purchase and registration dates, as well as any comments or additional information.

- **Price and Currency Management**: Keep track of the original and current prices of your items. Inventario supports multiple currencies, allowing you to monitor the value of your inventory in your preferred currency.

- **Attachments and Documentation**: Attach images, manuals, invoices, and other important documents to your items for easy reference and documentation.

- **User-friendly Interface**: Inventario offers a clean and intuitive interface that makes managing your inventory a breeze. The app is designed with a focus on simplicity and efficiency, ensuring that you can easily navigate and access all the necessary features.

- **Locations and Areas**: Organize your items into locations and areas to create a structured inventory. Define custom locations such as rooms, storage spaces, or any other relevant categories that suit your needs.

## Building and Running

Inventario is a Go application with a frontend built using web technologies. The following instructions will help you set up and run the application on your system.

### Prerequisites

- **Go**: Version 1.24 or higher
- **Node.js**: Version 22.15 or higher (managed via Volta)
- **Git**: For cloning the repository

### macOS

1. **Install prerequisites**:
   ```bash
   # Install Homebrew if not already installed
   /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
   
   # Install Go
   brew install go
   
   # Install Volta (Node.js version manager)
   brew install volta
   ```

2. **Clone and build the application**:
   ```bash
   git clone git@github.com:denisvmedia/inventario.git
   cd inventario
   make all
   ```

3. **Run the application**:
   ```bash
   cd bin && ./inventario run
   ```

4. **Seed the database** (optional, for development):
   ```bash
   curl -X POST http://localhost:3333/api/v1/seed
   ```

5. **Access the application**:
   Open your browser and navigate to http://localhost:3333/

### Linux

1. **Install prerequisites**:
   ```bash
   # Install Go (Ubuntu/Debian example)
   sudo apt update
   sudo apt install golang-go
   
   # Install Volta (Node.js version manager)
   curl https://get.volta.sh | bash
   source ~/.bashrc  # or restart your terminal
   ```

2. **Clone and build the application**:
   ```bash
   git clone git@github.com:denisvmedia/inventario.git
   cd inventario
   make all
   ```

3. **Run the application**:
   ```bash
   cd bin && ./inventario run
   ```

4. **Seed the database** (optional, for development):
   ```bash
   curl -X POST http://localhost:3333/api/v1/seed
   ```

5. **Access the application**:
   Open your browser and navigate to http://localhost:3333/

### Windows

1. **Install prerequisites**:
   - Install Go from [golang.org](https://golang.org/dl/)
   - Install Git from [git-scm.com](https://git-scm.com/download/win)
   - Install Volta using one of the following methods:
     - [Official installer](https://volta.sh/)
     - Using Scoop: `scoop install volta`
     - Using winget: `winget install volta.volta`

2. **Clone and build the application**:
   ```powershell
   git clone git@github.com:denisvmedia/inventario.git
   cd inventario
   make all
   ```
   Note: If you don't have Make installed, you can use Git Bash which includes Make.

3. **Run the application**:
   ```powershell
   cd bin
   .\inventario.exe run
   ```

4. **Seed the database** (optional, for development):
   ```powershell
   Invoke-RestMethod -Method POST -Uri "http://localhost:3333/api/v1/seed"
   ```
   or using curl if installed:
   ```
   curl -X POST http://localhost:3333/api/v1/seed
   ```

5. **Access the application**:
   Open your browser and navigate to http://localhost:3333/


## License
This module is licensed under the MIT License. See the [LICENSE](LICENSE) file for details. You are free to use, modify, and distribute this software in accordance with the terms of the license.

## Author

[Denis Voytyuk](https://github.com/denisvmedia)
